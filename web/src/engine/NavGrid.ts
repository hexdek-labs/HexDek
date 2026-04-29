// ── Directive: Idle Ops — Navigation Grid + A* Pathfinder ──

import type { BuildingLayout, LayoutRoom } from '../types';

export enum Tile {
  WALL = 0,
  FLOOR = 1,
  CORRIDOR = 2,
  DOOR = 3,
  EXTERIOR = 4,
}

interface GridPos {
  x: number;
  y: number;
}

// ── MinHeap for A* open set ──

class MinHeap {
  private data: { key: string; priority: number }[] = [];

  get size(): number {
    return this.data.length;
  }

  push(key: string, priority: number): void {
    this.data.push({ key, priority });
    this.bubbleUp(this.data.length - 1);
  }

  pop(): string {
    if (this.data.length === 0) throw new Error('Heap empty');
    const top = this.data[0];
    const last = this.data.pop()!;
    if (this.data.length > 0) {
      this.data[0] = last;
      this.sinkDown(0);
    }
    return top.key;
  }

  isEmpty(): boolean {
    return this.data.length === 0;
  }

  private bubbleUp(i: number): void {
    while (i > 0) {
      const parent = (i - 1) >> 1;
      if (this.data[parent].priority <= this.data[i].priority) break;
      [this.data[parent], this.data[i]] = [this.data[i], this.data[parent]];
      i = parent;
    }
  }

  private sinkDown(i: number): void {
    const n = this.data.length;
    while (true) {
      let smallest = i;
      const left = 2 * i + 1;
      const right = 2 * i + 2;
      if (left < n && this.data[left].priority < this.data[smallest].priority) smallest = left;
      if (right < n && this.data[right].priority < this.data[smallest].priority) smallest = right;
      if (smallest === i) break;
      [this.data[smallest], this.data[i]] = [this.data[i], this.data[smallest]];
      i = smallest;
    }
  }
}

// ── NavGrid ──

export class NavGrid {
  width: number;
  height: number;
  tiles: Tile[][];
  roomMap: number[][]; // [y][x] = room id, or -1

  constructor(layout: BuildingLayout) {
    // Scale: 1 layout unit = 2 grid cells (for doorway resolution)
    // +4 padding on each axis for exterior/rally area
    this.width = layout.width * 2 + 4;
    this.height = layout.height * 2 + 4;

    // Initialize all as WALL
    this.tiles = [];
    this.roomMap = [];
    for (let y = 0; y < this.height; y++) {
      this.tiles.push(new Array(this.width).fill(Tile.WALL));
      this.roomMap.push(new Array(this.width).fill(-1));
    }

    // Carve rooms as FLOOR
    for (const room of layout.rooms) {
      const gy1 = room.y * 2 + 2;
      const gx1 = room.x * 2 + 2;
      const gy2 = (room.y + room.h) * 2 + 2;
      const gx2 = (room.x + room.w) * 2 + 2;
      for (let gy = gy1; gy < gy2; gy++) {
        for (let gx = gx1; gx < gx2; gx++) {
          if (this.inBounds(gx, gy)) {
            this.tiles[gy][gx] = room.type === 'exterior' ? Tile.EXTERIOR : Tile.FLOOR;
            this.roomMap[gy][gx] = room.id;
          }
        }
      }
    }

    // Carve corridors between connected rooms
    for (const corridor of layout.corridors) {
      const roomA = layout.rooms.find(r => r.id === corridor.from);
      const roomB = layout.rooms.find(r => r.id === corridor.to);
      if (roomA && roomB) {
        this.carveCorridor(roomA, roomB);
      }
    }

    // Add exterior walkable strip along the bottom (rally area)
    // Row 0 and row 1 are exterior walkable area
    for (let gx = 0; gx < this.width; gx++) {
      this.tiles[0][gx] = Tile.EXTERIOR;
      this.tiles[1][gx] = Tile.EXTERIOR;
    }
  }

  private inBounds(x: number, y: number): boolean {
    return x >= 0 && x < this.width && y >= 0 && y < this.height;
  }

  private carveCorridor(roomA: LayoutRoom, roomB: LayoutRoom): void {
    // Convert rooms to grid coords (center points)
    const acx = roomA.x * 2 + 2 + roomA.w;
    const acy = roomA.y * 2 + 2 + roomA.h;
    const bcx = roomB.x * 2 + 2 + roomB.w;
    const bcy = roomB.y * 2 + 2 + roomB.h;

    // Find the grid-space bounding boxes
    const aLeft = roomA.x * 2 + 2;
    const aRight = (roomA.x + roomA.w) * 2 + 2 - 1;
    const aTop = roomA.y * 2 + 2;
    const aBottom = (roomA.y + roomA.h) * 2 + 2 - 1;

    const bLeft = roomB.x * 2 + 2;
    const bRight = (roomB.x + roomB.w) * 2 + 2 - 1;
    const bTop = roomB.y * 2 + 2;
    const bBottom = (roomB.y + roomB.h) * 2 + 2 - 1;

    // Determine if rooms share a horizontal or vertical wall
    const horizOverlap = Math.max(0, Math.min(aRight, bRight) - Math.max(aLeft, bLeft) + 1);
    const vertOverlap = Math.max(0, Math.min(aBottom, bBottom) - Math.max(aTop, bTop) + 1);

    if (horizOverlap >= 2) {
      // Rooms are vertically adjacent, shared horizontal wall
      const overlapStart = Math.max(aLeft, bLeft);
      const overlapEnd = Math.min(aRight, bRight);
      const mid = Math.floor((overlapStart + overlapEnd) / 2);

      // Which room is on top?
      const wallY = acx < bcx ? aBottom : bBottom; // wrong axis, fix:
      let wallRow: number;
      if (aBottom < bTop) {
        // roomA is above roomB
        wallRow = aBottom; // the row just below roomA's last row
        // Carve through the wall between them
        for (let gy = aBottom + 1; gy < bTop; gy++) {
          this.setTile(mid, gy, Tile.CORRIDOR);
          if (mid + 1 <= overlapEnd) this.setTile(mid + 1, gy, Tile.CORRIDOR);
        }
        // Door tiles at transitions
        this.setTile(mid, aBottom + 1, Tile.DOOR);
        if (bTop - 1 > aBottom + 1) this.setTile(mid, bTop - 1, Tile.DOOR);
      } else if (bBottom < aTop) {
        // roomB is above roomA
        for (let gy = bBottom + 1; gy < aTop; gy++) {
          this.setTile(mid, gy, Tile.CORRIDOR);
          if (mid + 1 <= overlapEnd) this.setTile(mid + 1, gy, Tile.CORRIDOR);
        }
        this.setTile(mid, bBottom + 1, Tile.DOOR);
        if (aTop - 1 > bBottom + 1) this.setTile(mid, aTop - 1, Tile.DOOR);
      } else {
        // Rooms overlap vertically -- they share a wall edge
        // Just put a door at the boundary
        void wallY; // unused, rooms overlap
        const sharedY = acy < bcy ? aBottom : bBottom;
        this.setTile(mid, sharedY, Tile.DOOR);
        this.setTile(mid, sharedY + 1, Tile.DOOR);
      }
    } else if (vertOverlap >= 2) {
      // Rooms are horizontally adjacent, shared vertical wall
      const overlapStart = Math.max(aTop, bTop);
      const overlapEnd = Math.min(aBottom, bBottom);
      const mid = Math.floor((overlapStart + overlapEnd) / 2);

      if (aRight < bLeft) {
        // roomA is to the left of roomB
        for (let gx = aRight + 1; gx < bLeft; gx++) {
          this.setTile(gx, mid, Tile.CORRIDOR);
          if (mid + 1 <= overlapEnd) this.setTile(gx, mid + 1, Tile.CORRIDOR);
        }
        this.setTile(aRight + 1, mid, Tile.DOOR);
        if (bLeft - 1 > aRight + 1) this.setTile(bLeft - 1, mid, Tile.DOOR);
      } else if (bRight < aLeft) {
        // roomB is to the left of roomA
        for (let gx = bRight + 1; gx < aLeft; gx++) {
          this.setTile(gx, mid, Tile.CORRIDOR);
          if (mid + 1 <= overlapEnd) this.setTile(gx, mid + 1, Tile.CORRIDOR);
        }
        this.setTile(bRight + 1, mid, Tile.DOOR);
        if (aLeft - 1 > bRight + 1) this.setTile(aLeft - 1, mid, Tile.DOOR);
      } else {
        // Rooms overlap horizontally
        const sharedX = acx < bcx ? aRight : bRight;
        this.setTile(sharedX, mid, Tile.DOOR);
        this.setTile(sharedX + 1, mid, Tile.DOOR);
      }
    } else {
      // Rooms not adjacent -- carve an L-shaped corridor through walls
      // Go horizontal first from A center, then vertical to B center
      const startX = acx;
      const startY = acy;
      const endX = bcx;
      const endY = bcy;

      // Horizontal segment
      const minX = Math.min(startX, endX);
      const maxX = Math.max(startX, endX);
      for (let gx = minX; gx <= maxX; gx++) {
        if (this.inBounds(gx, startY) && this.tiles[startY][gx] === Tile.WALL) {
          this.setTile(gx, startY, Tile.CORRIDOR);
        }
        if (this.inBounds(gx, startY + 1) && this.tiles[startY + 1][gx] === Tile.WALL) {
          this.setTile(gx, startY + 1, Tile.CORRIDOR);
        }
      }
      // Vertical segment
      const minY = Math.min(startY, endY);
      const maxY = Math.max(startY, endY);
      for (let gy = minY; gy <= maxY; gy++) {
        if (this.inBounds(endX, gy) && this.tiles[gy][endX] === Tile.WALL) {
          this.setTile(endX, gy, Tile.CORRIDOR);
        }
        if (this.inBounds(endX + 1, gy) && this.tiles[gy][endX + 1] === Tile.WALL) {
          this.setTile(endX + 1, gy, Tile.CORRIDOR);
        }
      }

      // Place DOOR tiles at room boundaries where corridor meets room
      this.placeDoorAtBoundary(roomA);
      this.placeDoorAtBoundary(roomB);
    }
  }

  /** Scan room perimeter for corridor tiles and convert them to DOOR */
  private placeDoorAtBoundary(room: LayoutRoom): void {
    const left = room.x * 2 + 2;
    const right = (room.x + room.w) * 2 + 2 - 1;
    const top = room.y * 2 + 2;
    const bottom = (room.y + room.h) * 2 + 2 - 1;

    // Check tiles just outside each edge
    const edges: [number, number][] = [];
    for (let gx = left; gx <= right; gx++) {
      edges.push([gx, top - 1]);
      edges.push([gx, bottom + 1]);
    }
    for (let gy = top; gy <= bottom; gy++) {
      edges.push([left - 1, gy]);
      edges.push([right + 1, gy]);
    }

    for (const [ex, ey] of edges) {
      if (this.inBounds(ex, ey) && this.tiles[ey][ex] === Tile.CORRIDOR) {
        this.tiles[ey][ex] = Tile.DOOR;
      }
    }
  }

  private setTile(x: number, y: number, tile: Tile): void {
    if (this.inBounds(x, y) && this.tiles[y][x] === Tile.WALL) {
      this.tiles[y][x] = tile;
    }
  }

  isWalkable(x: number, y: number): boolean {
    if (!this.inBounds(x, y)) return false;
    return this.tiles[y][x] !== Tile.WALL;
  }

  getRoomAt(x: number, y: number): number {
    if (!this.inBounds(x, y)) return -1;
    return this.roomMap[y][x];
  }

  getTileAt(x: number, y: number): Tile {
    if (!this.inBounds(x, y)) return Tile.WALL;
    return this.tiles[y][x];
  }

  // ── A* Pathfinding ──

  findPath(fromX: number, fromY: number, toX: number, toY: number): GridPos[] {
    if (!this.isWalkable(fromX, fromY) || !this.isWalkable(toX, toY)) {
      return [];
    }
    if (fromX === toX && fromY === toY) {
      return [{ x: toX, y: toY }];
    }

    const key = (x: number, y: number): string => `${x},${y}`;
    const heuristic = (ax: number, ay: number, bx: number, by: number): number => {
      // Octile distance for 8-directional movement
      const dx = Math.abs(ax - bx);
      const dy = Math.abs(ay - by);
      return Math.max(dx, dy) + (Math.SQRT2 - 1) * Math.min(dx, dy);
    };

    const start = key(fromX, fromY);
    const goal = key(toX, toY);

    const openSet = new MinHeap();
    const cameFrom = new Map<string, string>();
    const gScore = new Map<string, number>();

    gScore.set(start, 0);
    openSet.push(start, heuristic(fromX, fromY, toX, toY));

    const visited = new Set<string>();

    // 8-directional neighbors
    const dirs: [number, number][] = [
      [-1, 0], [1, 0], [0, -1], [0, 1],
      [-1, -1], [-1, 1], [1, -1], [1, 1],
    ];

    while (!openSet.isEmpty()) {
      const current = openSet.pop();
      if (current === goal) {
        return this.reconstructPath(cameFrom, current);
      }

      if (visited.has(current)) continue;
      visited.add(current);

      const [cx, cy] = current.split(',').map(Number);

      for (const [dx, dy] of dirs) {
        const nx = cx + dx;
        const ny = cy + dy;

        if (!this.isWalkable(nx, ny)) continue;

        // Prevent diagonal movement through wall corners
        if (dx !== 0 && dy !== 0) {
          if (!this.isWalkable(cx + dx, cy) || !this.isWalkable(cx, cy + dy)) {
            continue;
          }
        }

        const nKey = key(nx, ny);
        if (visited.has(nKey)) continue;

        const moveCost = (dx !== 0 && dy !== 0) ? Math.SQRT2 : 1;
        const tentativeG = (gScore.get(current) ?? Infinity) + moveCost;

        if (tentativeG < (gScore.get(nKey) ?? Infinity)) {
          cameFrom.set(nKey, current);
          gScore.set(nKey, tentativeG);
          const f = tentativeG + heuristic(nx, ny, toX, toY);
          openSet.push(nKey, f);
        }
      }
    }

    return []; // no path
  }

  private reconstructPath(cameFrom: Map<string, string>, current: string): GridPos[] {
    const path: GridPos[] = [];
    let node: string | undefined = current;
    while (node !== undefined) {
      const [x, y] = node.split(',').map(Number);
      path.unshift({ x, y });
      node = cameFrom.get(node);
    }
    return path;
  }

  // ── Spatial Queries ──

  /** All FLOOR tiles in a specific room */
  getFloorTilesInRoom(roomId: number): GridPos[] {
    const tiles: GridPos[] = [];
    for (let y = 0; y < this.height; y++) {
      for (let x = 0; x < this.width; x++) {
        if (this.roomMap[y][x] === roomId && this.tiles[y][x] === Tile.FLOOR) {
          tiles.push({ x, y });
        }
      }
    }
    return tiles;
  }

  /** DOOR tiles adjacent to a room */
  getDoorTilesForRoom(roomId: number): GridPos[] {
    const doors: GridPos[] = [];
    const seen = new Set<string>();
    for (let y = 0; y < this.height; y++) {
      for (let x = 0; x < this.width; x++) {
        if (this.tiles[y][x] !== Tile.DOOR) continue;
        // Check if any adjacent cell belongs to this room
        for (const [dx, dy] of [[-1, 0], [1, 0], [0, -1], [0, 1]] as [number, number][]) {
          const nx = x + dx;
          const ny = y + dy;
          if (this.inBounds(nx, ny) && this.roomMap[ny][nx] === roomId) {
            const k = `${x},${y}`;
            if (!seen.has(k)) {
              doors.push({ x, y });
              seen.add(k);
            }
            break;
          }
        }
      }
    }
    return doors;
  }

  /** Positions lined up in the corridor behind a door (for tactical stacking) */
  getStackPositions(doorX: number, doorY: number, count: number, fromRoomId: number): GridPos[] {
    // Find the corridor/exterior side of the door (away from the room)
    const positions: GridPos[] = [];
    let corridorDir: GridPos | null = null;

    for (const [dx, dy] of [[-1, 0], [1, 0], [0, -1], [0, 1]] as [number, number][]) {
      const nx = doorX + dx;
      const ny = doorY + dy;
      if (this.inBounds(nx, ny)) {
        const tile = this.tiles[ny][nx];
        const room = this.roomMap[ny][nx];
        if (room !== fromRoomId && (tile === Tile.CORRIDOR || tile === Tile.FLOOR || tile === Tile.EXTERIOR || tile === Tile.DOOR)) {
          corridorDir = { x: dx, y: dy };
          break;
        }
      }
    }

    if (!corridorDir) {
      // Fallback: just use the door position
      positions.push({ x: doorX, y: doorY });
      return positions;
    }

    // Walk along the corridor direction, collecting walkable positions
    for (let i = 0; i < count; i++) {
      const px = doorX + corridorDir.x * (i + 1);
      const py = doorY + corridorDir.y * (i + 1);
      if (this.isWalkable(px, py)) {
        positions.push({ x: px, y: py });
      } else {
        // Try perpendicular offsets
        const perpX = corridorDir.y;
        const perpY = corridorDir.x;
        const altX = doorX + corridorDir.x * (i + 1) + perpX;
        const altY = doorY + corridorDir.y * (i + 1) + perpY;
        if (this.isWalkable(altX, altY)) {
          positions.push({ x: altX, y: altY });
        } else {
          // Use door position as fallback
          positions.push({ x: doorX, y: doorY });
        }
      }
    }

    return positions;
  }

  /** Find a walkable cell near the entry area (bottom exterior strip) */
  getEntryPosition(index: number): GridPos {
    const gx = 2 + index * 2;
    if (this.isWalkable(gx, 1)) return { x: gx, y: 1 };
    if (this.isWalkable(gx, 0)) return { x: gx, y: 0 };
    // Scan for any exterior walkable tile
    for (let x = 0; x < this.width; x++) {
      if (this.tiles[0][x] === Tile.EXTERIOR) return { x, y: 0 };
      if (this.tiles[1][x] === Tile.EXTERIOR) return { x, y: 1 };
    }
    return { x: 0, y: 0 };
  }

  /** Get a random walkable position inside a room */
  getRandomFloorInRoom(roomId: number, rng: () => number): GridPos | null {
    const tiles = this.getFloorTilesInRoom(roomId);
    if (tiles.length === 0) return null;
    return tiles[Math.floor(rng() * tiles.length)];
  }

  // ── Coordinate Conversion ──

  /** Convert grid coords to layout coords (for rendering) */
  gridToLayout(gx: number, gy: number): { x: number; y: number } {
    return { x: (gx - 2) / 2, y: (gy - 2) / 2 };
  }

  /** Convert layout coords to grid coords */
  layoutToGrid(lx: number, ly: number): GridPos {
    return { x: Math.round(lx * 2 + 2), y: Math.round(ly * 2 + 2) };
  }

  /** Get the center of a room in grid coords */
  getRoomCenter(room: LayoutRoom): GridPos {
    const cx = room.x * 2 + 2 + room.w;
    const cy = room.y * 2 + 2 + room.h;
    // Clamp to a walkable tile near center
    if (this.isWalkable(cx, cy)) return { x: cx, y: cy };
    // Search outward
    for (let r = 1; r < 4; r++) {
      for (let dy = -r; dy <= r; dy++) {
        for (let dx = -r; dx <= r; dx++) {
          if (this.isWalkable(cx + dx, cy + dy) && this.roomMap[cy + dy]?.[cx + dx] === room.id) {
            return { x: cx + dx, y: cy + dy };
          }
        }
      }
    }
    // Last resort
    const tiles = this.getFloorTilesInRoom(room.id);
    return tiles.length > 0 ? tiles[0] : { x: cx, y: cy };
  }
}
