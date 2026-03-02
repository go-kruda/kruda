# Persona: Go Web Developer

## Primary Persona: Alex Chen
**Demographics:** 28 years old, Senior Backend Developer, San Francisco  
**Role:** Lead developer at a fintech startup, 5 years Go experience

## Goals
- Build high-performance APIs with minimal boilerplate
- Maintain type safety across request/response boundaries  
- Reduce time spent on repetitive validation and parsing code
- Ship features faster without sacrificing code quality
- Ensure production reliability and observability

## Frustrations
- "I spend too much time writing the same validation logic over and over"
- "Gin/Fiber require so much manual parsing - it's error-prone"
- "Type safety breaks down at HTTP boundaries"
- "Setting up proper error handling is tedious but critical"
- "Performance tuning requires deep framework knowledge"

## Behaviors
- Prefers explicit over implicit (Go philosophy)
- Values compile-time safety over runtime flexibility
- Reads framework source code to understand performance implications
- Contributes to open source projects
- Active in Go community (conferences, forums)

## Tech Comfort Level
**Expert** - Comfortable with Go generics, reflection, and performance optimization

## Key Quote
*"I want a framework that gets out of my way but catches my mistakes at compile time. If I can write less code and get better type safety, that's a win."*

## Scenario of Use
Building a REST API for payment processing that needs:
- Sub-100ms response times
- Comprehensive input validation
- Structured error responses for frontend
- Request tracing and monitoring
- Graceful degradation under load

## Device Preferences
- Primary: MacBook Pro with VS Code + Go extension
- Secondary: Linux server for deployment testing
- Mobile: GitHub app for code reviews

## Pain Points with Current Solutions
- **Gin:** Manual validation, verbose error handling, performance limitations
- **Fiber:** Express.js patterns don't feel "Go-like", limited type safety  
- **Echo:** Good performance but still requires lots of boilerplate
- **net/http:** Too low-level, missing modern conveniences

## What Attracts Them to Kruda
- Typed handlers `C[T]` eliminate manual parsing
- Auto-validation with struct tags
- Production-ready error handling out of the box
- Pluggable transport for performance optimization
- Zero-cost abstractions philosophy