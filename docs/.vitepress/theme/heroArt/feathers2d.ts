import { FEATHER_ASPECT, FEATHER_IMAGES, spawnFeather, stepFeather, type Feather } from './feather'

export function startFeathers2D(canvas: HTMLCanvasElement, card: HTMLElement | null, opts: { count: number; still: boolean }) {
  const ctx = canvas.getContext('2d')
  if (!ctx) return () => {}

  const images = FEATHER_IMAGES.map((src) => Object.assign(new Image(), { src, decoding: 'async' }))

  let w = 0
  let h = 0
  const edge = { x: 0.45, y: 0.55 }
  const resize = () => {
    const dpr = Math.min(devicePixelRatio || 1, 2)
    w = canvas.clientWidth
    h = canvas.clientHeight
    canvas.width = Math.round(w * dpr)
    canvas.height = Math.round(h * dpr)
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
    if (card && w && h) {
      edge.x = card.offsetWidth / w
      edge.y = card.offsetHeight / h
    }
  }
  resize()

  const feathers: Feather[] = Array.from({ length: opts.count }, () => spawnFeather(edge, true))

  const draw = () => {
    ctx.clearRect(0, 0, w, h)
    for (const f of feathers) {
      const img = images[f.image]
      if (!img.complete || !img.naturalWidth) continue
      const fh = f.size
      const fw = fh * FEATHER_ASPECT
      ctx.save()
      ctx.translate(w / 2 + f.x * w / 2, h / 2 + f.y * h / 2)
      ctx.rotate(f.angle)
      ctx.scale(f.turn, 1)
      ctx.globalAlpha = f.alpha * (0.6 + 0.4 * f.depth)
      ctx.drawImage(img, -fw / 2, -fh / 2, fw, fh)
      ctx.restore()
    }
  }

  if (opts.still) {
    Promise.all(images.map((i) => i.decode().catch(() => {}))).then(draw)
    return () => {}
  }

  let raf = 0
  let last = 0
  let visible = true
  const frame = (t: number) => {
    const dt = last ? Math.min((t - last) / 1000, 0.05) : 0
    last = t
    for (let i = 0; i < feathers.length; i++) {
      if (!stepFeather(feathers[i], dt)) feathers[i] = spawnFeather(edge, false)
    }
    draw()
    raf = requestAnimationFrame(frame)
  }
  const run = () => {
    cancelAnimationFrame(raf)
    last = 0
    if (visible && !document.hidden) raf = requestAnimationFrame(frame)
  }
  const io = new IntersectionObserver(([e]) => { visible = e.isIntersecting; run() })
  io.observe(canvas)
  document.addEventListener('visibilitychange', run)
  addEventListener('resize', resize)
  run()

  return () => {
    cancelAnimationFrame(raf)
    io.disconnect()
    document.removeEventListener('visibilitychange', run)
    removeEventListener('resize', resize)
  }
}
