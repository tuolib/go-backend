# Backend Engineer Career Roadmap

> A practical learning path for becoming a competitive backend engineer at global tech companies.
> Based on: TypeScript (completed) → Go (in progress) → production-ready engineer.

---

## Current Stack Assessment

| Skill | Status |
|-------|--------|
| TypeScript / Node.js | Completed — e-commerce platform with microservices |
| Go | In progress — rewriting the same project in Go |
| Frontend (React/Vue) | Can build with TS |

**Target combo: TypeScript (frontend/fullstack) + Go (backend) — one of the strongest skill pairs for a modern engineer.**

---

## Phase 1: Go Backend Proficiency (Now → 3-6 months)

### Goal: Ship a production-quality Go project

| Task | Details |
|------|---------|
| Complete Go e-commerce project | All 12 phases — auth, products, cart, orders, payments, stock control |
| Write thorough tests | Unit tests, integration tests, concurrency tests — global companies value this highly |
| Git workflow | PR-based flow, code review practices, CI integration |
| English codebase | All code comments, commit messages, and docs in English |
| Docker + basic DevOps | Containerization, compose, health checks |

### Key deliverables
- [ ] Fully working Go backend with 80%+ test coverage
- [ ] Clean Git history with English commit messages
- [ ] Docker-based deployment
- [ ] Architecture documentation in English

---

## Phase 2: Interview Core Skills (6-12 months)

### Priority ranking for global tech companies

```
1. English Communication     ████████████████████  Non-negotiable
2. Algorithms & Data Struct  ████████████████      Always tested
3. System Design             ██████████████        Required for mid/senior
4. Project Depth             ████████████          Why > What
5. Code Quality              ██████████            Clean, testable, designed
6. Specific Language         ██████                Go/Java/Python all fine
7. Framework Experience      ████                  Least important
```

### 2.1 English

| Activity | Time | Resource |
|----------|------|----------|
| Tech podcasts | 30 min/day | Go Time, Software Engineering Daily |
| Read English docs | Daily | Go official docs, RFCs, tech blogs |
| Write in English | Daily | Code comments, commit messages, READMEs |
| Mock interviews | Weekly | Practice explaining your system design in English |

### 2.2 Algorithms & Data Structures

| Topic | Priority | Target |
|-------|----------|--------|
| Arrays / Strings | High | Solve medium in 20 min |
| Hash Maps | High | Pattern recognition |
| Trees / Graphs | High | BFS, DFS, traversals |
| Dynamic Programming | High | Top-down + bottom-up |
| Linked Lists | Medium | In-place manipulation |
| Sorting / Searching | Medium | Know trade-offs |
| Sliding Window / Two Pointers | Medium | Common patterns |
| Greedy / Backtracking | Medium | When to apply |

**Daily practice: 1-2 LeetCode problems, target medium difficulty AC within 30 minutes.**

### 2.3 System Design

Use your e-commerce project as the foundation. Be ready to explain:

| Topic | Your project maps to |
|-------|---------------------|
| Load balancing | Caddy reverse proxy → multiple API servers |
| Caching strategy | Redis multi-level cache + singleflight + cache-aside |
| Database design | PG schema isolation, indexing strategy, connection pooling |
| Concurrency control | Redis Lua atomic stock deduction + optimistic locking |
| State machines | Order lifecycle (pending → paid → shipped → completed) |
| Idempotency | X-Idempotency-Key for order creation and payments |
| Auth design | JWT dual-token (access + refresh) + Redis blacklist |
| Rate limiting | Redis ZSET sliding window |
| API design | REST conventions, error code system, pagination |
| Microservices vs Monolith | Your dual-mode architecture (why, trade-offs) |

### 2.4 Go Deep Dive

| Topic | Why it matters |
|-------|---------------|
| Goroutine scheduler (GMP) | Interviewers love asking this |
| Garbage collector | Understanding pause times and tuning |
| Channel patterns | fan-in, fan-out, pipeline, context cancellation |
| sync primitives | Mutex, RWMutex, WaitGroup, Once, Pool |
| Memory model | Happens-before, race conditions |
| Performance profiling | pprof, benchmarks, escape analysis |
| Interface design | Implicit satisfaction, small interfaces |

### 2.5 SQL & Database

| Skill | Level needed |
|-------|-------------|
| Complex queries (JOIN, subquery, CTE) | Write fluently |
| EXPLAIN / query optimization | Read and optimize |
| Indexing strategies | Know when and why |
| Transactions & isolation levels | Understand trade-offs |
| Connection pooling | Configure and tune |

---

## Phase 3: Specialization (12+ months)

Choose ONE direction to go deep:

### Option A: Cloud Native (highest demand at global tech companies)

```
Kubernetes        → Deploy and manage your Go services on K8s
Terraform/Pulumi  → Infrastructure as Code
CI/CD             → GitHub Actions / GitLab CI pipeline
Observability     → Prometheus + Grafana + distributed tracing
Service Mesh      → Istio / Envoy (understand concepts)
```

### Option B: Data & Streaming

```
Kafka             → Event-driven architecture
Stream processing → Basic Flink/Spark concepts
Data pipelines    → ETL design patterns
PostgreSQL deep   → Partitioning, replication, JSONB
```

### Option C: Fullstack Product Engineer

```
React / Next.js   → Build the frontend for your Go API
React Native      → Mobile app
Vercel / Netlify   → Frontend deployment
End-to-end product → Design → Build → Deploy → Monitor
```

**Recommendation: Option A has the highest ROI for backend roles at global companies.**

---

## Do You Need Java?

| Your target | Learn Java? |
|-------------|-------------|
| Tech companies (Google, Meta, Stripe, etc.) | **No** — Go + Python is enough |
| Finance / Banking (Goldman Sachs, JPMorgan) | **Yes** — hard requirement |
| Consulting / IT services | **Light understanding** — engineering culture matters more |
| Startups / Scale-ups | **No** — they care about shipping, not specific languages |
| Not sure yet | **No** — don't invest until you have a specific target |

**Global tech companies care far more about problem-solving ability than how many languages you know.**

---

## What Global Companies Actually Evaluate

```
What matters:

  Problem solving    > Knowing algorithms by heart
  System thinking    > Memorizing design patterns
  Communication      > Writing perfect code
  Depth in one area  > Breadth across many
  Working code       > Theoretical knowledge
  Trade-off analysis > "Best practice" recitation
```

---

## Your Top 3 Actions Right Now

### 1. English (30 min/day)

Rewrite all your Go project documentation, comments, and commits in English. This is free practice that also makes your GitHub portfolio globally presentable.

### 2. Algorithms (1-2 problems/day)

LeetCode in Go. This reinforces both your algorithm skills and Go fluency simultaneously.

### 3. Finish the Go project

This is your system design interview material. When asked "design an e-commerce system", you won't be theorizing — you'll be speaking from experience.

---

## Timeline Summary

```
Month 0-6:    Go project completion + English habit + algorithm start
Month 6-12:   Interview prep (system design + algorithm grinding)
Month 12-18:  Specialization (cloud native recommended)
Month 18+:    Target applications, mock interviews, networking
```

---

## Portfolio Checklist

Before applying, ensure you have:

- [ ] Go e-commerce project on GitHub (English README, clean code, tests)
- [ ] Architecture documentation explaining design decisions
- [ ] 200+ LeetCode problems solved (Go solutions)
- [ ] English technical blog or detailed project write-ups (optional but strong signal)
- [ ] Open source contributions (even small ones — shows collaboration ability)

---

*Last updated: 2026-03-15*
