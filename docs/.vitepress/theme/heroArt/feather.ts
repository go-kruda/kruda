// Coordinates are normalised to the canvas: x and y run from -1 to 1 around
// the mascot's centre. `edge` is the mascot card's half-size in those units.

export const FEATHER_IMAGES = ['/feathers/feather-1.webp', '/feathers/feather-2.webp', '/feathers/feather-3.webp']
export const FEATHER_ASPECT = 256 / 384

export interface Feather {
  x: number
  y: number
  vy: number
  drift: number
  tilt: number
  phase: number
  rate: number
  rock: number
  size: number
  depth: number
  image: number
  age: number
  life: number
  alpha: number
  angle: number
  turn: number
}

const rand = (a: number, b: number) => a + Math.random() * (b - a)

// The mascot flies toward the upper right, so feathers peel off the card's
// left and bottom edges and drift down and to the left.
export function spawnFeather(edge: { x: number; y: number }, scatter: boolean): Feather {
  const depth = rand(0.55, 1)
  const life = rand(6, 10)
  const fromLeft = Math.random() < 0.55
  const x = fromLeft ? -edge.x * rand(0.8, 1) : rand(-edge.x, edge.x * 0.6)
  const y = fromLeft ? rand(-edge.y * 0.6, edge.y) : edge.y * rand(0.8, 1)
  const age = scatter ? rand(0, life * 0.7) : 0
  const vy = rand(0.04, 0.09)
  const drift = rand(-0.07, -0.02)
  const f: Feather = {
    x: x + drift * age,
    y: y + vy * age,
    vy,
    drift,
    tilt: rand(-0.5, 0.2),
    phase: rand(0, Math.PI * 2),
    rate: rand(1.6, 2.4),
    rock: rand(0.45, 0.8),
    size: rand(48, 72) * depth,
    depth,
    image: Math.floor(Math.random() * FEATHER_IMAGES.length),
    age,
    life,
    alpha: 0,
    angle: 0,
    turn: 1,
  }
  stepFeather(f, 0)
  return f
}

// A falling feather rocks like a pendulum: it slides sideways while it tilts
// and slows at each end of the swing, instead of tumbling like a leaf.
export function stepFeather(f: Feather, dt: number): boolean {
  f.age += dt
  if (f.age >= f.life) return false
  f.phase += f.rate * dt
  const swing = Math.sin(f.phase)
  f.x += (f.drift + Math.cos(f.phase) * 0.06) * dt
  f.y += f.vy * (0.6 + 0.4 * Math.abs(swing)) * dt
  f.angle = f.tilt + swing * f.rock
  f.turn = 0.7 + 0.3 * Math.cos(f.phase * 0.5)
  const t = f.age / f.life
  f.alpha = Math.min(1, t * 5, (1 - t) * 2.5)
  return true
}
