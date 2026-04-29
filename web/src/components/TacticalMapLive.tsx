// ── Directive: Idle Ops — Tactical Map Renderer ──
// Canvas-based renderer. Draws rooms as rectangles, corridors as paths,
// and positions entities at their grid coordinates converted to layout space.

import React, { useRef, useEffect, useCallback } from 'react';
import type { TacticalSim } from '../engine/TacticalSim';
import { Tile } from '../engine/NavGrid';
import type { SimOperator, SimOpfor, LayoutRoom } from '../types';

interface TacticalMapLiveProps {
  sim: TacticalSim;
}

// ── Colors ──

const COLORS = {
  bg: '#0f172a',
  wall: '#1e293b',
  floor: '#1e293b',
  floorStroke: '#334155',
  corridor: '#263040',
  door: '#475569',
  exterior: '#0c1322',
  roomCleared: 'rgba(74, 222, 128, 0.08)',
  roomActive: 'rgba(239, 68, 68, 0.12)',
  roomDefault: 'rgba(30, 41, 59, 0.6)',
  roomLabel: '#64748b',
  operatorAlive: '#4ade80',
  operatorDead: '#ef4444',
  opforAlive: '#ef4444',
  opforDead: '#64748b',
  gridLine: 'rgba(51, 65, 85, 0.3)',
  pathLine: 'rgba(96, 165, 250, 0.3)',
};

const ROLE_COLORS: Record<string, string> = {
  pointman: '#f59e0b',
  breacher: '#ef4444',
  support: '#4ade80',
  marksman: '#60a5fa',
};

export const TacticalMapLive: React.FC<TacticalMapLiveProps> = ({ sim }) => {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  // Compute scale and offset to fit the layout in the canvas
  const getTransform = useCallback((canvasW: number, canvasH: number) => {
    const layout = sim.layout;
    const padding = 40;
    // Add space for the exterior rally strip below
    const layoutW = layout.width;
    const layoutH = layout.height + 2; // extra for exterior strip

    const scaleX = (canvasW - padding * 2) / layoutW;
    const scaleY = (canvasH - padding * 2) / layoutH;
    const scale = Math.min(scaleX, scaleY);

    const offsetX = (canvasW - layoutW * scale) / 2;
    const offsetY = (canvasH - layoutH * scale) / 2;

    return { scale, offsetX, offsetY };
  }, [sim.layout]);

  // Convert layout coords to screen coords
  const toScreen = useCallback((lx: number, ly: number, scale: number, offsetX: number, offsetY: number) => {
    return {
      sx: offsetX + lx * scale,
      sy: offsetY + ly * scale,
    };
  }, []);

  // Draw frame
  const draw = useCallback(() => {
    const canvas = canvasRef.current;
    const container = containerRef.current;
    if (!canvas || !container) return;

    const rect = container.getBoundingClientRect();
    const dpr = window.devicePixelRatio || 1;
    canvas.width = rect.width * dpr;
    canvas.height = rect.height * dpr;
    canvas.style.width = `${rect.width}px`;
    canvas.style.height = `${rect.height}px`;

    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    ctx.scale(dpr, dpr);
    const cw = rect.width;
    const ch = rect.height;

    const { scale, offsetX, offsetY } = getTransform(cw, ch);

    // Clear
    ctx.fillStyle = COLORS.bg;
    ctx.fillRect(0, 0, cw, ch);

    // Draw exterior strip (rally area)
    ctx.fillStyle = COLORS.exterior;
    const extTopLeft = toScreen(0, -1, scale, offsetX, offsetY);
    ctx.fillRect(extTopLeft.sx, extTopLeft.sy, sim.layout.width * scale, 1 * scale);

    // Draw rooms
    for (const room of sim.layout.rooms) {
      const { sx, sy } = toScreen(room.x, room.y, scale, offsetX, offsetY);
      const w = room.w * scale;
      const h = room.h * scale;

      // Room fill
      if (sim.clearedRooms.has(room.id)) {
        ctx.fillStyle = COLORS.roomCleared;
      } else if (sim.isActiveRoom(room.id)) {
        ctx.fillStyle = COLORS.roomActive;
      } else {
        ctx.fillStyle = COLORS.roomDefault;
      }
      ctx.fillRect(sx, sy, w, h);

      // Room border
      ctx.strokeStyle = COLORS.floorStroke;
      ctx.lineWidth = 1;
      ctx.strokeRect(sx, sy, w, h);

      // Room label
      ctx.fillStyle = COLORS.roomLabel;
      ctx.font = `${Math.max(8, Math.min(12, scale * 0.4))}px 'JetBrains Mono', monospace`;
      ctx.textAlign = 'center';
      ctx.textBaseline = 'middle';
      ctx.fillText(room.label, sx + w / 2, sy + h / 2);
    }

    // Draw corridors (lines between connected rooms)
    ctx.strokeStyle = COLORS.corridor;
    ctx.lineWidth = Math.max(2, scale * 0.15);
    for (const corridor of sim.layout.corridors) {
      const roomA = sim.layout.rooms.find(r => r.id === corridor.from);
      const roomB = sim.layout.rooms.find(r => r.id === corridor.to);
      if (!roomA || !roomB) continue;

      const a = toScreen(roomA.x + roomA.w / 2, roomA.y + roomA.h / 2, scale, offsetX, offsetY);
      const b = toScreen(roomB.x + roomB.w / 2, roomB.y + roomB.h / 2, scale, offsetX, offsetY);

      ctx.beginPath();
      ctx.moveTo(a.sx, a.sy);
      ctx.lineTo(b.sx, b.sy);
      ctx.stroke();
    }

    // Draw doors on the grid
    for (let gy = 0; gy < sim.grid.height; gy++) {
      for (let gx = 0; gx < sim.grid.width; gx++) {
        if (sim.grid.getTileAt(gx, gy) === Tile.DOOR) {
          const lc = sim.grid.gridToLayout(gx, gy);
          const { sx, sy } = toScreen(lc.x, lc.y, scale, offsetX, offsetY);
          const tileSize = scale / 2;
          ctx.fillStyle = COLORS.door;
          ctx.fillRect(sx - tileSize / 2, sy - tileSize / 2, tileSize, tileSize);
        }
      }
    }

    // Draw operator paths (debug)
    ctx.strokeStyle = COLORS.pathLine;
    ctx.lineWidth = 1;
    ctx.setLineDash([2, 3]);
    for (const op of sim.operators) {
      if (!op.alive || op.path.length === 0 || op.pathIdx >= op.path.length) continue;
      ctx.beginPath();
      const startLC = sim.grid.gridToLayout(op.gridX, op.gridY);
      const start = toScreen(startLC.x, startLC.y, scale, offsetX, offsetY);
      ctx.moveTo(start.sx, start.sy);
      for (let i = op.pathIdx; i < op.path.length; i++) {
        const lc = sim.grid.gridToLayout(op.path[i].x, op.path[i].y);
        const p = toScreen(lc.x, lc.y, scale, offsetX, offsetY);
        ctx.lineTo(p.sx, p.sy);
      }
      ctx.stroke();
    }
    ctx.setLineDash([]);

    // Draw OPFOR
    for (const opf of sim.opfor) {
      drawOpfor(ctx, opf, scale, offsetX, offsetY, toScreen);
    }

    // Draw operators
    for (const op of sim.operators) {
      drawOperator(ctx, op, scale, offsetX, offsetY, toScreen);
    }

    // Cleared room checkmarks
    ctx.fillStyle = '#4ade80';
    ctx.font = `bold ${Math.max(10, scale * 0.5)}px sans-serif`;
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    for (const room of sim.layout.rooms) {
      if (sim.clearedRooms.has(room.id)) {
        const { sx, sy } = toScreen(room.x + room.w / 2, room.y + room.h / 2 + 0.5, scale, offsetX, offsetY);
        ctx.globalAlpha = 0.5;
        ctx.fillText('\u2713', sx, sy);
        ctx.globalAlpha = 1;
      }
    }
  }, [sim, getTransform, toScreen]);

  // Render loop via RAF (synced with sim updates via re-render)
  useEffect(() => {
    draw();
  });

  // Resize handler
  useEffect(() => {
    const onResize = () => draw();
    window.addEventListener('resize', onResize);
    return () => window.removeEventListener('resize', onResize);
  }, [draw]);

  return (
    <div ref={containerRef} style={{ width: '100%', height: '100%', position: 'relative' }}>
      <canvas ref={canvasRef} style={{ display: 'block' }} />
    </div>
  );
};

// ── Entity Drawing ──

type ToScreenFn = (lx: number, ly: number, scale: number, offsetX: number, offsetY: number) => { sx: number; sy: number };

function drawOperator(
  ctx: CanvasRenderingContext2D,
  op: SimOperator,
  scale: number,
  offsetX: number,
  offsetY: number,
  toScreen: ToScreenFn,
): void {
  const { sx, sy } = toScreen(op.x, op.y, scale, offsetX, offsetY);
  const radius = Math.max(3, scale * 0.18);

  if (!op.alive) {
    // Dead operator: X mark
    ctx.strokeStyle = COLORS.operatorDead;
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(sx - radius, sy - radius);
    ctx.lineTo(sx + radius, sy + radius);
    ctx.moveTo(sx + radius, sy - radius);
    ctx.lineTo(sx - radius, sy + radius);
    ctx.stroke();
    return;
  }

  // Alive operator: filled circle with role color ring
  const roleColor = ROLE_COLORS[op.role] || COLORS.operatorAlive;

  // Outer ring (role color)
  ctx.beginPath();
  ctx.arc(sx, sy, radius + 1.5, 0, Math.PI * 2);
  ctx.fillStyle = roleColor;
  ctx.fill();

  // Inner fill
  ctx.beginPath();
  ctx.arc(sx, sy, radius, 0, Math.PI * 2);
  ctx.fillStyle = COLORS.operatorAlive;
  ctx.fill();

  // Callsign label
  ctx.fillStyle = '#e2e8f0';
  ctx.font = `bold ${Math.max(7, scale * 0.12)}px 'JetBrains Mono', monospace`;
  ctx.textAlign = 'center';
  ctx.textBaseline = 'bottom';
  ctx.fillText(op.callsign, sx, sy - radius - 3);

  // State indicator for breaching/stacking
  if (op.state === 'breaching' || op.state === 'stacking') {
    ctx.strokeStyle = '#f59e0b';
    ctx.lineWidth = 1.5;
    ctx.setLineDash([2, 2]);
    ctx.beginPath();
    ctx.arc(sx, sy, radius + 4, 0, Math.PI * 2);
    ctx.stroke();
    ctx.setLineDash([]);
  }
}

function drawOpfor(
  ctx: CanvasRenderingContext2D,
  opf: SimOpfor,
  scale: number,
  offsetX: number,
  offsetY: number,
  toScreen: ToScreenFn,
): void {
  const { sx, sy } = toScreen(opf.x, opf.y, scale, offsetX, offsetY);
  const size = Math.max(3, scale * 0.15);

  if (!opf.alive) {
    // Dead: small dimmed X
    ctx.strokeStyle = COLORS.opforDead;
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(sx - size * 0.6, sy - size * 0.6);
    ctx.lineTo(sx + size * 0.6, sy + size * 0.6);
    ctx.moveTo(sx + size * 0.6, sy - size * 0.6);
    ctx.lineTo(sx - size * 0.6, sy + size * 0.6);
    ctx.stroke();
    return;
  }

  // Alive hostile: red diamond
  ctx.fillStyle = COLORS.opforAlive;
  ctx.beginPath();
  ctx.moveTo(sx, sy - size);
  ctx.lineTo(sx + size, sy);
  ctx.lineTo(sx, sy + size);
  ctx.lineTo(sx - size, sy);
  ctx.closePath();
  ctx.fill();

  // Type indicator for heavy/bomber
  if (opf.type === 'heavy') {
    ctx.strokeStyle = '#fff';
    ctx.lineWidth = 1;
    ctx.strokeRect(sx - size * 0.3, sy - size * 0.3, size * 0.6, size * 0.6);
  } else if (opf.type === 'bomber') {
    ctx.fillStyle = '#fbbf24';
    ctx.beginPath();
    ctx.arc(sx, sy, size * 0.3, 0, Math.PI * 2);
    ctx.fill();
  }
}
