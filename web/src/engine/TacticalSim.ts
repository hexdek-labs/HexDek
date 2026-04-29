// ── Directive: Idle Ops — Tactical Simulation Engine ──
// Grid-based movement via NavGrid + A* pathfinding. Operators can ONLY
// occupy walkable tiles and ONLY move along computed A* paths.

import { NavGrid, Tile } from './NavGrid';
import type {
  BuildingLayout,
  ScenarioData,
  SimOperator,
  SimOpfor,
  SimPhase,
  EventLogEntry,
  SimResult,
  LayoutRoom,
} from '../types';

// ── Seeded PRNG (xorshift32) ──

function makeRng(seed: number): () => number {
  let s = seed | 0 || 1;
  return () => {
    s ^= s << 13;
    s ^= s >> 17;
    s ^= s << 5;
    return (s >>> 0) / 4294967296;
  };
}

// ── Room clear order planner ──

function planClearOrder(layout: BuildingLayout): number[] {
  // BFS from entry room through corridors
  const visited = new Set<number>();
  const order: number[] = [];
  const queue: number[] = [layout.entryRoomId];
  visited.add(layout.entryRoomId);

  // Build adjacency from corridors
  const adj = new Map<number, number[]>();
  for (const room of layout.rooms) {
    adj.set(room.id, []);
  }
  for (const c of layout.corridors) {
    adj.get(c.from)?.push(c.to);
    adj.get(c.to)?.push(c.from);
  }

  while (queue.length > 0) {
    const roomId = queue.shift()!;
    order.push(roomId);
    const neighbors = adj.get(roomId) || [];
    for (const n of neighbors) {
      if (!visited.has(n)) {
        visited.add(n);
        queue.push(n);
      }
    }
  }

  // Add any unreachable rooms at the end
  for (const room of layout.rooms) {
    if (!visited.has(room.id)) {
      order.push(room.id);
    }
  }

  return order;
}

// ── TacticalSim ──

export class TacticalSim {
  // Public state (consumed by OperationLive / TacticalMapLive)
  grid: NavGrid;
  operators: SimOperator[];
  opfor: SimOpfor[];
  layout: BuildingLayout;
  phase: SimPhase;
  eventLog: EventLogEntry[];
  clearedRooms: Set<number>;
  activeRooms: Set<number>;
  elapsed: number;
  timeLimit: number;

  // Internal
  private rng: () => number;
  private clearOrder: number[];
  private clearIndex: number;
  private currentTargetRoomId: number;
  private objectiveRoomIds: Set<number>;
  private breachTimer: number;
  private combatTimer: number;
  private phaseTimer: number;
  private opforIdCounter: number;
  private scenario: ScenarioData;

  // Movement timing
  private static readonly MOVE_INTERVAL = 0.12; // seconds per grid cell
  private static readonly BREACH_DURATION = 1.5; // seconds to breach a door
  private static readonly CLEAR_DURATION = 2.0; // seconds to clear a room after combat
  private static readonly ENGAGEMENT_INTERVAL = 0.8; // seconds between combat rolls

  constructor(scenario: ScenarioData, seed?: number) {
    this.scenario = scenario;
    this.layout = scenario.layout;
    this.grid = new NavGrid(scenario.layout);
    this.rng = makeRng(seed ?? Date.now());
    this.elapsed = 0;
    this.timeLimit = scenario.timeLimit;
    this.phase = 'planning';
    this.eventLog = [];
    this.clearedRooms = new Set();
    this.activeRooms = new Set();
    this.breachTimer = 0;
    this.combatTimer = 0;
    this.phaseTimer = 0;
    this.opforIdCounter = 0;

    // Plan room clear order
    this.clearOrder = planClearOrder(scenario.layout);
    this.clearIndex = 0;
    this.currentTargetRoomId = this.clearOrder[0] ?? -1;
    this.objectiveRoomIds = new Set(scenario.objectiveRoomIds);

    // Initialize operators at entry positions
    this.operators = scenario.operators.map((def, i) => {
      const entryPos = this.grid.getEntryPosition(i);
      return {
        id: def.id,
        callsign: def.callsign,
        role: def.role,
        hp: def.hp,
        maxHp: def.hp,
        accuracy: def.accuracy,
        speed: def.speed,
        alive: true,
        x: 0,
        y: 0,
        gridX: entryPos.x,
        gridY: entryPos.y,
        path: [],
        pathIdx: 0,
        state: 'idle' as const,
        currentRoomId: -1,
        moveAccum: 0,
      };
    });
    // Sync layout coords
    for (const op of this.operators) {
      const lc = this.grid.gridToLayout(op.gridX, op.gridY);
      op.x = lc.x;
      op.y = lc.y;
    }

    // Initialize OPFOR in their rooms
    this.opfor = [];
    for (const placement of scenario.opforPlacements) {
      const room = scenario.layout.rooms.find(r => r.id === placement.roomId);
      if (!room) continue;
      for (let i = 0; i < placement.count; i++) {
        const pos = this.grid.getRandomFloorInRoom(placement.roomId, this.rng);
        if (!pos) continue;
        const opf: SimOpfor = {
          id: `opfor-${this.opforIdCounter++}`,
          type: placement.type,
          hp: placement.type === 'heavy' ? 150 : placement.type === 'bomber' ? 60 : 100,
          maxHp: placement.type === 'heavy' ? 150 : placement.type === 'bomber' ? 60 : 100,
          alive: true,
          x: 0,
          y: 0,
          gridX: pos.x,
          gridY: pos.y,
          roomId: placement.roomId,
          state: 'idle',
          accuracy: placement.type === 'heavy' ? 0.35 : placement.type === 'bomber' ? 0.2 : 0.3,
        };
        const lc = this.grid.gridToLayout(pos.x, pos.y);
        opf.x = lc.x;
        opf.y = lc.y;
        this.opfor.push(opf);
      }
    }

    this.log('info', 'Scenario loaded: ' + scenario.name);
    this.log('info', `${this.operators.length} operators, ${this.opfor.length} hostiles`);
    this.log('info', `Grid: ${this.grid.width}x${this.grid.height} tiles`);
  }

  // ── Public API ──

  get isComplete(): boolean {
    return this.phase === 'complete' || this.phase === 'failed';
  }

  get currentRoom(): number {
    return this.currentTargetRoomId;
  }

  get aliveOperators(): SimOperator[] {
    return this.operators.filter(op => op.alive);
  }

  isActiveRoom(roomId: number): boolean {
    return this.activeRooms.has(roomId);
  }

  getResult(): SimResult {
    const totalOpfor = this.scenario.opforPlacements.reduce((sum, p) => sum + p.count, 0);
    const clearableRooms = this.layout.rooms.filter(r => r.type === 'room').length;
    return {
      success: this.phase === 'complete',
      operatorsAlive: this.aliveOperators.length,
      operatorsTotal: this.operators.length,
      opforNeutralized: this.opfor.filter(o => !o.alive).length,
      opforTotal: totalOpfor,
      roomsCleared: this.clearedRooms.size,
      roomsTotal: clearableRooms,
      elapsed: this.elapsed,
      events: [...this.eventLog],
    };
  }

  // ── Main Tick ──

  tick(dt: number): void {
    if (this.isComplete) return;

    this.elapsed += dt;

    // Time limit check
    if (this.elapsed >= this.timeLimit) {
      this.phase = 'failed';
      this.log('phase', 'MISSION FAILED - Time limit exceeded');
      return;
    }

    // All operators dead check
    if (this.aliveOperators.length === 0) {
      this.phase = 'failed';
      this.log('casualty', 'MISSION FAILED - All operators KIA');
      return;
    }

    switch (this.phase) {
      case 'planning':
        this.tickPlanning(dt);
        break;
      case 'infiltrating':
        this.tickInfiltrating(dt);
        break;
      case 'breaching':
        this.tickBreaching(dt);
        break;
      case 'clearing':
        this.tickClearing(dt);
        break;
      case 'objective':
        this.tickObjective(dt);
        break;
      case 'exfiltrating':
        this.tickExfiltrating(dt);
        break;
    }

    // Sync layout coords for rendering
    for (const op of this.operators) {
      if (!op.alive) continue;
      const lc = this.grid.gridToLayout(op.gridX, op.gridY);
      op.x = lc.x;
      op.y = lc.y;
      op.currentRoomId = this.grid.getRoomAt(op.gridX, op.gridY);
    }
    for (const opf of this.opfor) {
      if (!opf.alive) continue;
      const lc = this.grid.gridToLayout(opf.gridX, opf.gridY);
      opf.x = lc.x;
      opf.y = lc.y;
    }
  }

  // ── Phase Logic ──

  private tickPlanning(_dt: number): void {
    this.phaseTimer += _dt;
    if (this.phaseTimer >= 1.0) {
      this.phase = 'infiltrating';
      this.phaseTimer = 0;
      this.log('phase', 'Phase: INFILTRATING');
      this.orderMoveToRoom(this.currentTargetRoomId);
    }
  }

  private tickInfiltrating(dt: number): void {
    this.moveOperators(dt);

    // Check if all operators are near the target room's door
    const alive = this.aliveOperators;
    if (alive.length === 0) return;

    const allArrived = alive.every(op => op.path.length === 0 || op.pathIdx >= op.path.length);
    if (allArrived) {
      // Check if there are hostiles in the target room
      const hostiles = this.opfor.filter(o => o.alive && o.roomId === this.currentTargetRoomId);
      if (hostiles.length > 0) {
        this.phase = 'breaching';
        this.breachTimer = 0;
        this.phaseTimer = 0;
        this.activeRooms.add(this.currentTargetRoomId);
        this.log('phase', `Phase: BREACHING room "${this.getRoomLabel(this.currentTargetRoomId)}"`);
        // Set operators to stacking state
        for (const op of alive) {
          op.state = 'stacking';
        }
      } else {
        // Room is empty, mark cleared and move to next
        this.clearedRooms.add(this.currentTargetRoomId);
        this.activeRooms.delete(this.currentTargetRoomId);
        this.log('info', `Room "${this.getRoomLabel(this.currentTargetRoomId)}" clear (empty)`);
        this.advanceToNextRoom();
      }
    }
  }

  private tickBreaching(dt: number): void {
    this.breachTimer += dt;

    // Operators prepare at the door
    const alive = this.aliveOperators;
    for (const op of alive) {
      op.state = 'breaching';
    }

    if (this.breachTimer >= TacticalSim.BREACH_DURATION) {
      this.phase = 'clearing';
      this.combatTimer = 0;
      this.log('phase', `Phase: CLEARING room "${this.getRoomLabel(this.currentTargetRoomId)}"`);
      // Move operators into the room
      this.orderEnterRoom(this.currentTargetRoomId);
      for (const op of alive) {
        op.state = 'clearing';
      }
    }
  }

  private tickClearing(dt: number): void {
    this.moveOperators(dt);

    // Combat engagement
    this.combatTimer += dt;
    if (this.combatTimer >= TacticalSim.ENGAGEMENT_INTERVAL) {
      this.combatTimer -= TacticalSim.ENGAGEMENT_INTERVAL;
      this.resolveCombatRound(this.currentTargetRoomId);
    }

    // Check if all hostiles in room are dead
    const hostiles = this.opfor.filter(o => o.alive && o.roomId === this.currentTargetRoomId);
    if (hostiles.length === 0) {
      this.phaseTimer += dt;
      if (this.phaseTimer >= TacticalSim.CLEAR_DURATION) {
        this.clearedRooms.add(this.currentTargetRoomId);
        this.activeRooms.delete(this.currentTargetRoomId);
        this.log('info', `Room "${this.getRoomLabel(this.currentTargetRoomId)}" CLEAR`);

        // Check if this was an objective room
        if (this.objectiveRoomIds.has(this.currentTargetRoomId)) {
          this.objectiveRoomIds.delete(this.currentTargetRoomId);
          this.log('objective', `Objective secured in "${this.getRoomLabel(this.currentTargetRoomId)}"`);
        }

        this.advanceToNextRoom();
      }
    } else {
      this.phaseTimer = 0;
    }
  }

  private tickObjective(dt: number): void {
    this.phaseTimer += dt;
    if (this.phaseTimer >= 2.0) {
      this.log('objective', 'All objectives secured');
      this.phase = 'exfiltrating';
      this.phaseTimer = 0;
      this.log('phase', 'Phase: EXFILTRATING');
      // Move back to entry
      this.orderMoveToEntry();
    }
  }

  private tickExfiltrating(dt: number): void {
    this.moveOperators(dt);

    const alive = this.aliveOperators;
    const allExfiled = alive.every(op => {
      const tile = this.grid.getTileAt(op.gridX, op.gridY);
      return (tile === Tile.EXTERIOR) && (op.path.length === 0 || op.pathIdx >= op.path.length);
    });

    if (allExfiled) {
      this.phase = 'complete';
      this.log('phase', 'MISSION COMPLETE');
      for (const op of alive) {
        op.state = 'idle';
      }
    }
  }

  // ── Movement ──

  private moveOperators(dt: number): void {
    for (const op of this.operators) {
      if (!op.alive) continue;
      if (op.path.length === 0 || op.pathIdx >= op.path.length) {
        op.state = op.state === 'clearing' ? 'clearing' : 'idle';
        continue;
      }

      op.state = op.state === 'clearing' ? 'clearing' : 'moving';
      op.moveAccum += dt;

      const moveInterval = TacticalSim.MOVE_INTERVAL / op.speed;

      while (op.moveAccum >= moveInterval && op.pathIdx < op.path.length) {
        op.moveAccum -= moveInterval;
        const next = op.path[op.pathIdx];

        // CRITICAL: Never move to a non-walkable tile
        if (this.grid.isWalkable(next.x, next.y)) {
          op.gridX = next.x;
          op.gridY = next.y;
        }
        op.pathIdx++;
      }
    }
  }

  /** Order all alive operators to move near a room's door */
  private orderMoveToRoom(roomId: number): void {
    const room = this.layout.rooms.find(r => r.id === roomId);
    if (!room) return;

    const doors = this.grid.getDoorTilesForRoom(roomId);
    const alive = this.aliveOperators;

    if (doors.length > 0) {
      // Stack operators near the first door
      const door = doors[0];
      const stackPositions = this.grid.getStackPositions(door.x, door.y, alive.length, roomId);

      alive.forEach((op, i) => {
        const target = stackPositions[i] || stackPositions[stackPositions.length - 1] || door;
        const path = this.grid.findPath(op.gridX, op.gridY, target.x, target.y);
        op.path = path;
        op.pathIdx = 0;
        op.moveAccum = 0;
        op.state = 'moving';
      });
    } else {
      // No doors found -- move to room center directly
      const center = this.grid.getRoomCenter(room);
      for (const op of alive) {
        const path = this.grid.findPath(op.gridX, op.gridY, center.x, center.y);
        op.path = path;
        op.pathIdx = 0;
        op.moveAccum = 0;
        op.state = 'moving';
      }
    }
  }

  /** Order operators into a room (after breach) */
  private orderEnterRoom(roomId: number): void {
    const room = this.layout.rooms.find(r => r.id === roomId);
    if (!room) return;

    const alive = this.aliveOperators;
    const floorTiles = this.grid.getFloorTilesInRoom(roomId);
    if (floorTiles.length === 0) return;

    // Spread operators across room floor tiles
    alive.forEach((op, i) => {
      const target = floorTiles[i % floorTiles.length];
      const path = this.grid.findPath(op.gridX, op.gridY, target.x, target.y);
      op.path = path;
      op.pathIdx = 0;
      op.moveAccum = 0;
    });
  }

  /** Order operators back to the entry/exterior */
  private orderMoveToEntry(): void {
    const alive = this.aliveOperators;
    alive.forEach((op, i) => {
      const entry = this.grid.getEntryPosition(i);
      const path = this.grid.findPath(op.gridX, op.gridY, entry.x, entry.y);
      op.path = path;
      op.pathIdx = 0;
      op.moveAccum = 0;
      op.state = 'moving';
    });
  }

  // ── Combat ──

  private resolveCombatRound(roomId: number): void {
    const alive = this.aliveOperators;
    const hostiles = this.opfor.filter(o => o.alive && o.roomId === roomId);

    if (hostiles.length === 0 || alive.length === 0) return;

    // Operators shoot at hostiles
    for (const op of alive) {
      if (hostiles.length === 0) break;
      const target = hostiles[Math.floor(this.rng() * hostiles.length)];
      if (!target.alive) continue;

      const roll = this.rng();
      // Role bonus
      const bonus = op.role === 'pointman' ? 0.1 : op.role === 'marksman' ? 0.15 : 0;
      if (roll < op.accuracy + bonus) {
        const damage = 25 + Math.floor(this.rng() * 30);
        target.hp -= damage;
        if (target.hp <= 0) {
          target.hp = 0;
          target.alive = false;
          target.state = 'dead';
          this.log('combat', `${op.callsign} neutralized ${target.type} [${target.id}]`);
          // Remove from active list
          const idx = hostiles.indexOf(target);
          if (idx >= 0) hostiles.splice(idx, 1);
        }
      }
    }

    // Hostiles shoot back
    for (const hostile of hostiles) {
      if (!hostile.alive || alive.length === 0) continue;
      hostile.state = 'engaging';
      const target = alive[Math.floor(this.rng() * alive.length)];
      if (!target.alive) continue;

      const roll = this.rng();
      // Operators in clearing state have a defensive advantage
      const defBonus = target.state === 'clearing' ? -0.1 : 0;
      if (roll < hostile.accuracy + defBonus) {
        const damage = 15 + Math.floor(this.rng() * 20);
        target.hp -= damage;
        if (target.hp <= 0) {
          target.hp = 0;
          target.alive = false;
          target.state = 'dead';
          target.path = [];
          this.log('casualty', `${target.callsign} KIA by ${hostile.type}`);
        } else {
          this.log('combat', `${target.callsign} hit by ${hostile.type} (${target.hp}/${target.maxHp} HP)`);
        }
      }
    }
  }

  // ── Room Progression ──

  private advanceToNextRoom(): void {
    this.clearIndex++;
    this.phaseTimer = 0;

    // Check if all objectives are done
    if (this.objectiveRoomIds.size === 0) {
      this.phase = 'objective';
      this.phaseTimer = 0;
      return;
    }

    // Find next uncleared room
    while (this.clearIndex < this.clearOrder.length) {
      const nextRoomId = this.clearOrder[this.clearIndex];
      if (!this.clearedRooms.has(nextRoomId)) {
        this.currentTargetRoomId = nextRoomId;
        this.phase = 'infiltrating';
        this.log('phase', `Phase: INFILTRATING → "${this.getRoomLabel(nextRoomId)}"`);
        this.orderMoveToRoom(nextRoomId);
        return;
      }
      this.clearIndex++;
    }

    // All rooms cleared
    if (this.objectiveRoomIds.size === 0) {
      this.phase = 'objective';
      this.phaseTimer = 0;
    } else {
      // Objectives not yet secured -- this shouldn't happen if layout is valid
      this.phase = 'failed';
      this.log('phase', 'MISSION FAILED - Cannot reach remaining objectives');
    }
  }

  // ── Helpers ──

  private getRoomLabel(roomId: number): string {
    const room = this.layout.rooms.find(r => r.id === roomId);
    return room?.label || `Room ${roomId}`;
  }

  private log(type: EventLogEntry['type'], text: string): void {
    this.eventLog.push({
      time: Math.round(this.elapsed * 10) / 10,
      text,
      type,
    });
  }
}
