// ── Directive: Idle Ops — Shared Types ──

// ── Building / Layout ──

export interface LayoutRoom {
  id: number;
  x: number;
  y: number;
  w: number;
  h: number;
  label: string;
  type: 'room' | 'hallway' | 'stairwell' | 'exterior';
}

export interface Corridor {
  from: number; // room id
  to: number;   // room id
}

export interface BuildingLayout {
  width: number;
  height: number;
  rooms: LayoutRoom[];
  corridors: Corridor[];
  entryRoomId: number; // which room is the building entry
}

// ── Scenario ──

export interface ScenarioData {
  id: string;
  name: string;
  description: string;
  layout: BuildingLayout;
  operators: OperatorDef[];
  opforPlacements: OpforPlacement[];
  objectiveRoomIds: number[];
  timeLimit: number; // seconds
}

export interface OperatorDef {
  id: string;
  callsign: string;
  role: 'pointman' | 'breacher' | 'support' | 'marksman';
  hp: number;
  accuracy: number; // 0-1
  speed: number;    // cells per second on the grid
}

export interface OpforPlacement {
  roomId: number;
  count: number;
  type: 'guard' | 'heavy' | 'bomber';
}

// ── Sim Entities ──

export interface SimOperator {
  id: string;
  callsign: string;
  role: 'pointman' | 'breacher' | 'support' | 'marksman';
  hp: number;
  maxHp: number;
  accuracy: number;
  speed: number;
  alive: boolean;
  x: number; // layout coords (for rendering)
  y: number;
  gridX: number; // grid coords (for pathfinding)
  gridY: number;
  path: { x: number; y: number }[]; // A* path in grid coords
  pathIdx: number;
  state: 'idle' | 'moving' | 'stacking' | 'breaching' | 'clearing' | 'holding' | 'dead';
  currentRoomId: number;
  moveAccum: number; // accumulated time for grid movement
}

export interface SimOpfor {
  id: string;
  type: 'guard' | 'heavy' | 'bomber';
  hp: number;
  maxHp: number;
  alive: boolean;
  x: number;
  y: number;
  gridX: number;
  gridY: number;
  roomId: number;
  state: 'idle' | 'alert' | 'engaging' | 'dead';
  accuracy: number;
}

// ── Sim State ──

export type SimPhase =
  | 'planning'
  | 'infiltrating'
  | 'breaching'
  | 'clearing'
  | 'objective'
  | 'exfiltrating'
  | 'complete'
  | 'failed';

export interface EventLogEntry {
  time: number;
  text: string;
  type: 'info' | 'combat' | 'objective' | 'casualty' | 'phase';
}

export interface SimResult {
  success: boolean;
  operatorsAlive: number;
  operatorsTotal: number;
  opforNeutralized: number;
  opforTotal: number;
  roomsCleared: number;
  roomsTotal: number;
  elapsed: number;
  events: EventLogEntry[];
}
