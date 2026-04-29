// ── Directive: Idle Ops — Operation Live View ──
// Orchestrates the TacticalSim and renders the tactical map + HUD.

import React, { useEffect, useRef, useState, useCallback } from 'react';
import { TacticalSim } from '../engine/TacticalSim';
import { TacticalMapLive } from './TacticalMapLive';
import type { ScenarioData, SimResult } from '../types';

interface OperationLiveProps {
  scenario: ScenarioData;
  seed?: number;
  tickRate?: number; // ticks per second (default 30)
  onComplete?: (result: SimResult) => void;
}

export const OperationLive: React.FC<OperationLiveProps> = ({
  scenario,
  seed,
  tickRate = 30,
  onComplete,
}) => {
  const simRef = useRef<TacticalSim | null>(null);
  const rafRef = useRef<number>(0);
  const lastTimeRef = useRef<number>(0);
  const [, forceUpdate] = useState(0);
  const onCompleteRef = useRef(onComplete);
  onCompleteRef.current = onComplete;

  // Initialize sim
  useEffect(() => {
    simRef.current = new TacticalSim(scenario, seed);
    lastTimeRef.current = 0;
    forceUpdate(n => n + 1);

    return () => {
      if (rafRef.current) cancelAnimationFrame(rafRef.current);
    };
  }, [scenario, seed]);

  // Animation loop
  useEffect(() => {
    const sim = simRef.current;
    if (!sim) return;

    const dt = 1 / tickRate;
    let accumulator = 0;

    const loop = (timestamp: number) => {
      if (lastTimeRef.current === 0) lastTimeRef.current = timestamp;
      const frameDt = Math.min((timestamp - lastTimeRef.current) / 1000, 0.1); // cap at 100ms
      lastTimeRef.current = timestamp;
      accumulator += frameDt;

      let ticked = false;
      while (accumulator >= dt) {
        sim.tick(dt);
        accumulator -= dt;
        ticked = true;
      }

      if (ticked) {
        forceUpdate(n => n + 1);
      }

      if (sim.isComplete) {
        onCompleteRef.current?.(sim.getResult());
        return; // stop the loop
      }

      rafRef.current = requestAnimationFrame(loop);
    };

    rafRef.current = requestAnimationFrame(loop);

    return () => {
      if (rafRef.current) cancelAnimationFrame(rafRef.current);
    };
  }, [tickRate]);

  const sim = simRef.current;
  if (!sim) return null;

  return (
    <div style={styles.container}>
      {/* Header */}
      <div style={styles.header}>
        <div style={styles.missionTitle}>{scenario.name}</div>
        <div style={styles.phaseIndicator}>
          <span style={styles.phaseLabel}>PHASE</span>
          <span style={{
            ...styles.phaseValue,
            color: phaseColor(sim.phase),
          }}>
            {sim.phase.toUpperCase()}
          </span>
        </div>
        <div style={styles.timer}>
          {formatTime(sim.elapsed)} / {formatTime(sim.timeLimit)}
        </div>
      </div>

      {/* Main content: Map + Sidebar */}
      <div style={styles.main}>
        <div style={styles.mapContainer}>
          <TacticalMapLive sim={sim} />
        </div>

        <div style={styles.sidebar}>
          {/* Operator status */}
          <div style={styles.panel}>
            <div style={styles.panelTitle}>OPERATORS</div>
            {sim.operators.map(op => (
              <div
                key={op.id}
                style={{
                  ...styles.operatorRow,
                  opacity: op.alive ? 1 : 0.4,
                }}
              >
                <div style={styles.opCallsign}>{op.callsign}</div>
                <div style={styles.opRole}>{op.role}</div>
                <div style={styles.opHpBar}>
                  <div
                    style={{
                      ...styles.opHpFill,
                      width: `${(op.hp / op.maxHp) * 100}%`,
                      backgroundColor: op.hp > op.maxHp * 0.5 ? '#4ade80' : op.hp > op.maxHp * 0.25 ? '#facc15' : '#ef4444',
                    }}
                  />
                </div>
                <div style={styles.opState}>{op.state}</div>
              </div>
            ))}
          </div>

          {/* Room status */}
          <div style={styles.panel}>
            <div style={styles.panelTitle}>ROOMS</div>
            {sim.layout.rooms
              .filter(r => r.type === 'room')
              .map(room => {
                const cleared = sim.clearedRooms.has(room.id);
                const active = sim.isActiveRoom(room.id);
                const hostileCount = sim.opfor.filter(o => o.alive && o.roomId === room.id).length;
                return (
                  <div
                    key={room.id}
                    style={{
                      ...styles.roomRow,
                      backgroundColor: active ? 'rgba(239, 68, 68, 0.15)' : cleared ? 'rgba(74, 222, 128, 0.1)' : 'transparent',
                    }}
                  >
                    <span style={styles.roomLabel}>{room.label}</span>
                    <span style={{
                      ...styles.roomStatus,
                      color: cleared ? '#4ade80' : active ? '#ef4444' : '#94a3b8',
                    }}>
                      {cleared ? 'CLEAR' : active ? `ACTIVE (${hostileCount})` : hostileCount > 0 ? `${hostileCount} hostile` : '--'}
                    </span>
                  </div>
                );
              })}
          </div>

          {/* Event log */}
          <div style={{ ...styles.panel, flex: 1, overflow: 'hidden' }}>
            <div style={styles.panelTitle}>EVENT LOG</div>
            <div style={styles.eventLog}>
              {[...sim.eventLog].reverse().slice(0, 50).map((evt, i) => (
                <div key={i} style={{
                  ...styles.eventEntry,
                  color: eventColor(evt.type),
                }}>
                  <span style={styles.eventTime}>[{evt.time.toFixed(1)}s]</span>
                  {evt.text}
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>

      {/* Result overlay */}
      {sim.isComplete && <ResultOverlay sim={sim} />}
    </div>
  );
};

// ── Result Overlay ──

const ResultOverlay: React.FC<{ sim: TacticalSim }> = ({ sim }) => {
  const result = sim.getResult();
  return (
    <div style={styles.overlay}>
      <div style={styles.overlayContent}>
        <div style={{
          ...styles.resultTitle,
          color: result.success ? '#4ade80' : '#ef4444',
        }}>
          {result.success ? 'MISSION COMPLETE' : 'MISSION FAILED'}
        </div>
        <div style={styles.resultStats}>
          <div>Operators: {result.operatorsAlive}/{result.operatorsTotal} alive</div>
          <div>OPFOR: {result.opforNeutralized}/{result.opforTotal} neutralized</div>
          <div>Rooms: {result.roomsCleared}/{result.roomsTotal} cleared</div>
          <div>Time: {formatTime(result.elapsed)}</div>
        </div>
      </div>
    </div>
  );
};

// ── Helpers ──

function formatTime(seconds: number): string {
  const m = Math.floor(seconds / 60);
  const s = Math.floor(seconds % 60);
  return `${m}:${s.toString().padStart(2, '0')}`;
}

function phaseColor(phase: string): string {
  switch (phase) {
    case 'planning': return '#94a3b8';
    case 'infiltrating': return '#60a5fa';
    case 'breaching': return '#f59e0b';
    case 'clearing': return '#ef4444';
    case 'objective': return '#a78bfa';
    case 'exfiltrating': return '#34d399';
    case 'complete': return '#4ade80';
    case 'failed': return '#ef4444';
    default: return '#94a3b8';
  }
}

function eventColor(type: string): string {
  switch (type) {
    case 'combat': return '#f59e0b';
    case 'casualty': return '#ef4444';
    case 'objective': return '#a78bfa';
    case 'phase': return '#60a5fa';
    default: return '#cbd5e1';
  }
}

// ── Styles ──

const styles: Record<string, React.CSSProperties> = {
  container: {
    display: 'flex',
    flexDirection: 'column',
    width: '100%',
    height: '100vh',
    backgroundColor: '#0f172a',
    color: '#e2e8f0',
    fontFamily: "'JetBrains Mono', 'Fira Code', monospace",
    overflow: 'hidden',
  },
  header: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: '8px 16px',
    backgroundColor: '#1e293b',
    borderBottom: '1px solid #334155',
  },
  missionTitle: {
    fontSize: '14px',
    fontWeight: 700,
    textTransform: 'uppercase' as const,
    letterSpacing: '1px',
  },
  phaseIndicator: {
    display: 'flex',
    alignItems: 'center',
    gap: '8px',
  },
  phaseLabel: {
    fontSize: '10px',
    color: '#64748b',
  },
  phaseValue: {
    fontSize: '14px',
    fontWeight: 700,
  },
  timer: {
    fontSize: '14px',
    color: '#94a3b8',
    fontVariantNumeric: 'tabular-nums',
  },
  main: {
    display: 'flex',
    flex: 1,
    overflow: 'hidden',
  },
  mapContainer: {
    flex: 1,
    position: 'relative' as const,
    overflow: 'hidden',
  },
  sidebar: {
    width: '280px',
    display: 'flex',
    flexDirection: 'column',
    borderLeft: '1px solid #334155',
    backgroundColor: '#1e293b',
    overflow: 'hidden',
  },
  panel: {
    padding: '8px 12px',
    borderBottom: '1px solid #334155',
  },
  panelTitle: {
    fontSize: '10px',
    fontWeight: 700,
    color: '#64748b',
    letterSpacing: '1px',
    marginBottom: '6px',
  },
  operatorRow: {
    display: 'flex',
    alignItems: 'center',
    gap: '6px',
    padding: '3px 0',
    fontSize: '11px',
  },
  opCallsign: {
    fontWeight: 700,
    width: '60px',
  },
  opRole: {
    color: '#64748b',
    width: '60px',
    fontSize: '10px',
  },
  opHpBar: {
    flex: 1,
    height: '4px',
    backgroundColor: '#334155',
    borderRadius: '2px',
    overflow: 'hidden',
  },
  opHpFill: {
    height: '100%',
    borderRadius: '2px',
    transition: 'width 0.3s',
  },
  opState: {
    width: '60px',
    textAlign: 'right' as const,
    fontSize: '9px',
    color: '#94a3b8',
    textTransform: 'uppercase' as const,
  },
  roomRow: {
    display: 'flex',
    justifyContent: 'space-between',
    padding: '2px 4px',
    fontSize: '11px',
    borderRadius: '2px',
  },
  roomLabel: {
    color: '#e2e8f0',
  },
  roomStatus: {
    fontSize: '10px',
    fontWeight: 700,
  },
  eventLog: {
    overflowY: 'auto' as const,
    maxHeight: '300px',
    fontSize: '10px',
    lineHeight: '1.4',
  },
  eventEntry: {
    padding: '1px 0',
  },
  eventTime: {
    color: '#475569',
    marginRight: '4px',
  },
  overlay: {
    position: 'absolute' as const,
    inset: 0,
    backgroundColor: 'rgba(0, 0, 0, 0.7)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    zIndex: 100,
  },
  overlayContent: {
    textAlign: 'center' as const,
    padding: '32px',
  },
  resultTitle: {
    fontSize: '28px',
    fontWeight: 700,
    letterSpacing: '2px',
    marginBottom: '16px',
  },
  resultStats: {
    fontSize: '14px',
    color: '#94a3b8',
    lineHeight: '1.8',
  },
};
