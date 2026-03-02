# UI Designer Handoff: Kruda Developer Experience

## Project Overview
Design interfaces and experiences that support Kruda framework adoption and developer productivity. Focus on documentation, error pages, and developer tooling interfaces.

## User Flow Diagrams
- **New Developer Onboarding:** `docs/ux/journeys/new-developer-onboarding.md`
- **Framework Migration:** `docs/ux/journeys/framework-migration.md`

## Key Screens to Design

### 1. Dev Mode Error Page
**Purpose:** Rich HTML error page for development debugging  
**Context:** Displays when errors occur in development mode

**Content Hierarchy:**
1. **Primary:** Error type and message (large, prominent)
2. **Secondary:** Stack trace with file links
3. **Tertiary:** Request details, source code context
4. **Actions:** Copy error, view docs, restart server

**Interaction Specs:**
- **Hover:** File paths become clickable links to IDE
- **Click:** Stack trace items expand/collapse for detail
- **Copy:** One-click error copying for sharing/reporting
- **Responsive:** Works on laptop screens (primary) and tablets

**Visual Requirements:**
- **Syntax highlighting** for Go code snippets
- **Dark/light theme** support (detect system preference)
- **Monospace fonts** for code, sans-serif for UI text
- **Color coding:** Errors (red), warnings (yellow), info (blue)

### 2. Documentation Landing Page
**Purpose:** First impression for developers discovering Kruda  
**Context:** Primary entry point from GitHub, search, referrals

**Content Hierarchy:**
1. **Hero:** Value proposition, quick start code example
2. **Benefits:** Key differentiators vs competitors
3. **Examples:** Live code samples with copy buttons
4. **Getting Started:** Installation and first steps
5. **Navigation:** Clear path to detailed guides

**Interaction Specs:**
- **Code examples:** Syntax highlighted, copy-to-clipboard
- **Tabs:** Switch between framework comparisons
- **Progressive disclosure:** Expand/collapse sections
- **Search:** Instant search across all documentation

### 3. Interactive Playground
**Purpose:** Try Kruda without local installation  
**Context:** Learning tool for new developers

**Layout:**
- **Left panel:** Code editor with syntax highlighting
- **Right panel:** Output/response display
- **Bottom panel:** Console logs and errors
- **Top bar:** Example selector, share button

**Interaction Specs:**
- **Real-time:** Code changes trigger immediate execution
- **Examples:** Dropdown with common patterns
- **Share:** Generate shareable URLs for code snippets
- **Export:** Download as complete Go project

## Wireframe Descriptions

### Dev Error Page Layout
```
┌─────────────────────────────────────────────────┐
│ [Kruda Logo] Development Error                  │
├─────────────────────────────────────────────────┤
│                                                 │
│ ⚠️  ValidationError                            │
│     Field 'email' validation failed            │
│                                                 │
│ ┌─────────────────────────────────────────────┐ │
│ │ Request Details                             │ │
│ │ POST /api/users                             │ │
│ │ Content-Type: application/json              │ │
│ └─────────────────────────────────────────────┘ │
│                                                 │
│ ┌─────────────────────────────────────────────┐ │
│ │ Stack Trace                                 │ │
│ │ > main.go:42 createUser()                   │ │
│ │   kruda.go:156 (*App).handleError()         │ │
│ │   context.go:89 (*Ctx).Next()               │ │
│ └─────────────────────────────────────────────┘ │
│                                                 │
│ ┌─────────────────────────────────────────────┐ │
│ │ Source Context                              │ │
│ │ 40: func createUser(c *kruda.C[User]) {     │ │
│ │ 41:   if err := validate(c.In); err != nil │ │
│ │ 42: ►   return err                          │ │
│ │ 43: }                                       │ │
│ └─────────────────────────────────────────────┘ │
│                                                 │
│ [Copy Error] [View Docs] [Restart Server]      │
└─────────────────────────────────────────────────┘
```

### Documentation Landing Page Layout
```
┌─────────────────────────────────────────────────┐
│ [Navigation Bar]                    [Search]    │
├─────────────────────────────────────────────────┤
│                                                 │
│           🚀 Kruda Framework                    │
│     Type-safe Go web framework                  │
│        with auto-everything                     │
│                                                 │
│ ┌─────────────────┐ ┌─────────────────────────┐ │
│ │ Quick Start     │ │ // Hello World          │ │
│ │                 │ │ app := kruda.New()      │ │
│ │ go get kruda    │ │ app.Get("/", handler)   │ │
│ │ [Copy]          │ │ app.Listen(":3000")     │ │
│ └─────────────────┘ └─────────────────────────┘ │
│                                                 │
│ ┌─────────────────────────────────────────────┐ │
│ │ ✨ 60% Less Boilerplate                     │ │
│ │ 🔒 Compile-time Type Safety                 │ │
│ │ ⚡ Zero-cost Abstractions                   │ │
│ │ 🔧 Production-ready Error Handling          │ │
│ └─────────────────────────────────────────────┘ │
│                                                 │
│ [Get Started] [Examples] [Documentation]       │
└─────────────────────────────────────────────────┘
```

## Accessibility Requirements

### WCAG 2.1 AA Compliance
- **Color contrast:** 4.5:1 minimum for normal text, 3:1 for large text
- **Keyboard navigation:** All interactive elements accessible via keyboard
- **Screen readers:** Semantic HTML, proper ARIA labels
- **Focus indicators:** Clear visual focus states for all controls

### Component-Specific Requirements

#### Dev Error Page
- **Keyboard navigation:** Tab through stack trace items
- **Screen reader:** Error details announced clearly
- **Color contrast:** Error states meet contrast requirements
- **Touch targets:** Minimum 44x44px for mobile debugging

#### Documentation
- **Skip links:** Jump to main content, navigation
- **Heading structure:** Proper H1-H6 hierarchy
- **Code blocks:** Screen reader friendly with language labels
- **Search:** Keyboard accessible with clear results

#### Interactive Playground  
- **Code editor:** Screen reader compatible editor
- **Keyboard shortcuts:** Standard editor shortcuts supported
- **Error announcements:** Compilation errors announced to screen readers
- **Output formatting:** Structured for assistive technology

## Edge Cases & Error States

### Dev Error Page
- **Long stack traces:** Truncate with expand option
- **Large source files:** Show context window around error line
- **Network errors:** Graceful fallback when IDE links fail
- **Mobile viewing:** Responsive layout for tablet debugging

### Documentation
- **Search no results:** Helpful suggestions and popular pages
- **Broken examples:** Clear error messages with fix suggestions
- **Slow loading:** Progressive loading with skeleton screens
- **Offline access:** Service worker for core documentation

### Interactive Playground
- **Compilation errors:** Clear error highlighting in editor
- **Runtime panics:** Graceful error display without breaking UI
- **Network timeouts:** Retry mechanism with user feedback
- **Large outputs:** Pagination or truncation with full view option

## Content Priority & Hierarchy

### Primary Content (Must See)
- Value proposition and key benefits
- Working code examples
- Getting started instructions
- Error messages and solutions

### Secondary Content (Should See)
- Detailed API documentation
- Migration guides
- Performance benchmarks
- Community resources

### Tertiary Content (Nice to Have)
- Advanced configuration options
- Contribution guidelines
- Roadmap and changelog
- Related projects

## Success Metrics

### User Engagement
- **Time on documentation:** > 3 minutes average
- **Code example interactions:** > 50% copy rate
- **Search usage:** < 30% bounce rate from search
- **Error page actions:** > 20% click-through to docs

### Developer Experience
- **Task completion:** > 90% successful first-time setup
- **Error resolution:** < 5 minutes average debug time
- **Satisfaction scores:** > 4.5/5 for documentation clarity
- **Return usage:** > 60% return within 7 days