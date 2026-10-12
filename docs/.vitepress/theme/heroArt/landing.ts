import { FEATHER_ASPECT, FEATHER_IMAGES } from './feather'

const REST_ANGLE = 12

// One feather leaves the mascot, drifts up and lands at the end of the hero
// name, where `slot` (a zero-width span after "Kruda") marks the baseline.
export function landFeather(slot: HTMLElement, card: HTMLElement | null, opts: { still: boolean }) {
  const rest = makeFeather()
  rest.classList.add('kruda-feather-rest')
  slot.appendChild(rest)

  if (opts.still || !card || !rest.animate) {
    rest.style.opacity = '1'
    return () => rest.remove()
  }

  const flier = makeFeather()
  Object.assign(flier.style, { position: 'absolute', left: '0', top: '0', zIndex: '30', opacity: '0' })
  document.body.appendChild(flier)

  let animation: Animation | undefined
  let timer = 0
  const land = () => {
    animation?.cancel()
    rest.style.opacity = '1'
    flier.remove()
    removeEventListener('resize', onResize)
  }
  let width = 0
  const onResize = () => { if (innerWidth !== width) land() }
  const fly = () => {
    width = innerWidth
    const h = rest.offsetHeight
    const w = h * FEATHER_ASPECT
    flier.style.height = `${h}px`
    const s = slot.getBoundingClientRect()
    const c = card.getBoundingClientRect()
    const end = { x: s.left + rest.offsetLeft + scrollX, y: s.top - h + scrollY }
    const from = { x: c.left + c.width * 0.1 - w / 2 + scrollX, y: c.top + c.height * 0.55 - h / 2 + scrollY }
    const c1 = { x: from.x - 90, y: from.y - 170 }
    const c2 = { x: end.x + 140, y: end.y - 130 }

    const frames: Keyframe[] = []
    const steps = 60
    for (let i = 0; i <= steps; i++) {
      const u = i / steps
      const t = 0.5 - Math.cos(Math.PI * u) / 2
      const k = 1 - t
      const x = k ** 3 * from.x + 3 * k * k * t * c1.x + 3 * k * t * t * c2.x + t ** 3 * end.x
      const y = k ** 3 * from.y + 3 * k * k * t * c1.y + 3 * k * t * t * c2.y + t ** 3 * end.y
      const sway = Math.sin(u * Math.PI * 3)
      const angle = -25 * k + REST_ANGLE * t * t + sway * 40 * k
      frames.push({
        offset: u,
        opacity: Math.min(1, u * 10),
        transform: `translate(${x + sway * 36 * k}px, ${y}px) rotate(${angle}deg) scale(${0.8 + 0.2 * t})`,
      })
    }
    animation = flier.animate(frames, { duration: 2800, easing: 'linear', fill: 'both' })
    animation.onfinish = land
    addEventListener('resize', onResize)
  }

  // Positions are measured right before take-off, once fonts have settled and
  // the tab is visible, so a late layout shift cannot strand the flight.
  const whenVisible = () => {
    if (!document.hidden) return fly()
    document.addEventListener('visibilitychange', whenVisible, { once: true })
  }
  const loaded = rest.complete ? Promise.resolve() : new Promise((r) => rest.addEventListener('load', r, { once: true }))
  Promise.all([loaded, document.fonts?.ready]).then(() => {
    timer = window.setTimeout(whenVisible, 900)
  })

  return () => {
    clearTimeout(timer)
    document.removeEventListener('visibilitychange', whenVisible)
    removeEventListener('resize', onResize)
    animation?.cancel()
    flier.remove()
    rest.remove()
  }
}

function makeFeather() {
  const img = new Image()
  img.src = FEATHER_IMAGES[0]
  img.alt = ''
  img.decoding = 'async'
  img.className = 'kruda-feather-img'
  img.style.transformOrigin = '50% 100%'
  return img
}

