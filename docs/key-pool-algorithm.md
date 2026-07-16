# Adaptive Decaying Confidence Scheduler

> A stateless adaptive scheduler that models resource reliability as a **decaying posterior confidence**, balancing exploitation and recovery through continuous feedback — no discrete state machine.

Implementation: [`internal/loadbalance/loadbalance.go`](../internal/loadbalance/loadbalance.go) · Config: [`etc/matrix.yaml`](../etc/matrix.yaml)

---

## Overview

The Amap provider maintains a **multi-key pool**. Each `Route()` picks one key; on failure it tries the next. Outcomes are recorded via `RecordSuccess` / `RecordFailure`.

The scheduler addresses four problems at once:

| Problem | Approach |
|---------|----------|
| Should we try an unknown key? | Optimistic prior $\pi{=}1$, Beta $\rho_0{\approx}0.67$ |
| QPS throttle vs permanently dead key? | Soft / hard failure split; hard-only → probe-only mode |
| Should dead keys be reprobed? | Time gate $g(\Delta t)$ + exponential backoff $T(F_b)$ |
| Can a lone key starve? | $\varepsilon(N)$ active only when $N{\le}1$ |

**State variables** (all continuous, no FSM):

$$
S,\; F_{\text{soft}},\; F_{\text{hard}},\; \Delta t,\; N
$$

**Output**: selection rate $R \in [0,1]$ → Smooth WRR weight $W = \text{base} \cdot R \cdot 1000$.

Document layout: **Complete formula** (one-page cheat sheet) → **Walkthrough** (section by section) → simulation & references.

---

## Complete formula

### Outcome decay

$$
S' = S \cdot e^{-\lambda \Delta t}
$$

$$
F' = F \cdot e^{-\lambda \Delta t}, \quad F = F_{\text{soft}} + F_{\text{hard}}
$$

$$
\lambda = \frac{\ln 2}{\text{KeyRecoverySec}}
$$

(Soft/hard split is applied **before** decay on record; see [§2 Failure decomposition](#2-failure-decomposition).)

---

### Failure split (`RecordFailure`)

$$
F = F_{\text{soft}} + F_{\text{hard}}
$$

- If $S > 0$: $F_{\text{soft}} {+}{=} 1$ (e.g. QPS throttle after prior success)
- Else: $F_{\text{hard}} {+}{=} 1$ (e.g. IP block, recycled key)

$$
F_b = F_{\text{hard}} + w_f \cdot F_{\text{soft}}
$$

---

### Selection rate (single chain)

$$
\rho = \frac{S + a}{S + F + a + b}
\qquad\text{—— Beta posterior; } a{=}2,\; b{=}1,\; \rho_0 {=} \tfrac{2}{3}
$$

$$
\eta = S + w_f \cdot F
\qquad\text{—— weighted sample mass (failures count less toward confidence)}
$$

$$
\alpha = 1 - e^{-\eta / \tau}
\qquad\text{—— when to trust history}
$$

$$
R_0 = (1-\alpha)\,\pi + \alpha\,\rho
\qquad\text{—— shrinkage; } \pi {=} 1.0
$$

$$
T(F_b) = T_0 \cdot e^{F_b / \phi}
\qquad\text{—— more failures → longer probe interval}
$$

$$
g(\Delta t) = 1 - e^{-\Delta t / T(F_b)}
\qquad\text{—— just probed } {\to}\, 0 \text{; waited long enough } {\to}\, 1
$$

$$
\varepsilon(N) = \begin{cases} e^{-N/k} & N \le 1 \\ 0 & N > 1 \end{cases}
\qquad\text{—— lone-key anti-starvation; zero for multi-key pools}
$$

$$
g' = g + (1-g)\,\varepsilon(N)
$$

**Normal key** (has success history, or $F_{\text{hard}} < 1$):

$$
R = R_0 + \rho_{\min} \cdot g' \cdot (1 - R_0)
$$

**Hard-fail key** ($S {=} 0$ and $F_{\text{hard}} \ge 1$, probe-only):

$$
R = \begin{cases}
0 & g' {=} 0 \\
\rho_{\min} \cdot g' \cdot (1 - R_0) & g' > 0
\end{cases}
$$

$$
W = \text{base} \cdot R \cdot 1000
\qquad\text{—— Smooth WRR weight}
$$

$\Delta t$ = time since last **success or failure** (`lastOutcomeAt`), not since last WRR pick.

---

### Parameters

| Param | Default | Meaning |
|-------|---------|---------|
| $T_0$ | **30s** (`KeyProbeSec`) | Base probe interval |
| $\phi$ | **5** | Failure backoff scale; $F_b{=}5$ → interval $\times e$ |
| $\rho_{\min}$ | **0.02** | Peak probe weight |
| $\pi$ | **1.0** | Unexplored prior (shrinkage term) |
| $a, b$ | **2, 1** | Beta prior; new key $\rho_0 \approx 0.67$ |
| $w_f$ | **0.3** | Failure weight in $\eta$ and $F_b$ |
| $\tau$ | **2.0** (`KeyConfidenceTau`) | Confidence ramp speed (configurable) |
| $k$ | **4** (`KeyEpsilonScale`) | $\varepsilon = e^{-N/k}$ |
| halfLife | **300s** (`KeyRecoverySec`) | Outcome decay half-life |

---

### How behavior emerges

- **Unexplored key** ($\eta{=}0$): $\alpha{=}0 \Rightarrow R_0{=}\pi{=}1$ — full weight.
- **Good key** (QPS throttle ~32% success): $R_0 \approx \rho$; failures → $F_{\text{soft}}$, not probe-only.
- **Dead key just probed** ($\Delta t{=}0$): $g{=}0$; multi-key $\varepsilon{=}0 \Rightarrow g'{=}0$; probe-only → $R{\approx}0$.
- **Dead key after $T(F_b)$**: $g \to 1$, $R \to \rho_{\min}(1-R_0) \approx 0.02$ — periodic probe.
- **More failures**: $T(F_b)$ grows exponentially — longer backoff.
- **Lone key pool**: $\varepsilon(1) {=} e^{-1/4} \approx 0.78$ — probe path when gate closed.
- **Time decay**: $F_{\text{hard}} < 1$ exits probe-only; key treated as unexplored again.

---

## Walkthrough

### Design philosophy

| Principle | Meaning |
|-----------|---------|
| **Continuous state** | `S`, `F_soft`, `F_hard`, `Δt`, `N` replace `healthy/degraded/disabled` |
| **Outcome-based** | Any error counts the same at record time; soft/hard split uses **history**, not infocode |
| **Restless bandit** | Arms decay, backoff, and reprobe over time — not a static bandit |
| **No FSM** | All transitions are implicit in one formula chain |

---

### 1. Outcome decay

Historical counters decay exponentially (half-life = `KeyRecoverySec`). The cheat sheet uses aggregate $F$; the implementation decays $F_{\text{soft}}$ and $F_{\text{hard}}$ separately with the same $\lambda$:

$$
S' = S \cdot e^{-\lambda \Delta t}, \quad
F_{\text{soft}}' = F_{\text{soft}} \cdot e^{-\lambda \Delta t}, \quad
F_{\text{hard}}' = F_{\text{hard}} \cdot e^{-\lambda \Delta t}
$$

$$
\lambda = \frac{\ln 2}{\text{KeyRecoverySec}}
$$

Lets keys that failed in the past but may have recovered become explorable again.

---

### 2. Failure decomposition

On `RecordFailure`:

- If $S > 0$ (recent success): increment $F_{\text{soft}}$ — e.g. QPS throttle mixed with OK
- Else: increment $F_{\text{hard}}$ — likely dead / IP / recycled key

$$
F = F_{\text{soft}} + F_{\text{hard}}
$$

Backoff uses weighted failures:

$$
F_b = F_{\text{hard}} + w_f \cdot F_{\text{soft}}, \quad w_f = 0.3
$$

**Probe-only mode**: when $S{=}0$ and $F_{\text{hard}} \ge 1$, skip $R_0$ exploitation; when gate open $R \approx \rho_{\min} g'$, when closed $R{=}0$. Below one effective hard failure, key is unexplored again.

---

### 3. Posterior success rate (Beta smoothing)

Optimistic prior $\text{Beta}(a{=}2,\, b{=}1)$ — unexplored keys start at $\rho \approx 0.67$, not $0.5$:

$$
\rho = \frac{S + a}{S + F + a + b}
$$

With $a{=}2$, $b{=}1$: $\rho = \dfrac{S+2}{S+F+3}$.

Legacy $\rho = S/(S+F)$ gives $\rho{=}1$ at $S{=}1,F{=}0$ (overfit on tiny samples). Beta aligns with optimistic $\pi{=}1$: **unknown keys default to worth trying**.

---

### 4. Confidence ramp (when to trust history)

Failures contribute less to confidence than successes:

$$
\eta = S + w_f \cdot F
$$

$$
\alpha = 1 - e^{-\eta / \tau}, \quad \tau = 2 \text{ (configurable)}
$$

$\alpha$ controls exploit vs prior $\pi$ only; $\rho$ itself remains asymmetric via Beta.

$w_f{=}0.3$ avoids interpreting “100 failures, zero success” as high confidence ($\eta$ too large → $\alpha$ locks in too fast).

---

### 5. Shrinkage reward

$$
R_0 = (1-\alpha)\,\pi + \alpha\,\rho, \quad \pi = 1.0
$$

Unexplored ($\eta{=}0$): $\alpha{=}0 \Rightarrow R_0{=}\pi{=}1$ (full weight).

---

### 6. Time-gated probe (backoff)

$$
T(F_b) = T_0 \cdot e^{F_b / \phi}
$$

$$
g(\Delta t) = 1 - e^{-\Delta t / T(F_b)}
$$

Just probed ($\Delta t{=}0$): $g{=}0$. After waiting $\gg T(F_b)$: $g \to 1$.

$T_0 = \texttt{KeyProbeSec}$ (default 30s), $\phi = 5$.

---

### 7. Single-key anti-starvation

Exploration comes from $g(\Delta t)$, not from $\varepsilon$. $\varepsilon$ only prevents $R{=}0$ when $N{=}1$:

$$
\varepsilon(N) = \begin{cases} e^{-N/k} & N \le 1 \\ 0 & N > 1 \end{cases}, \quad k = 4
$$

$$
g' = g + (1-g)\,\varepsilon(N)
$$

Multi-key pools use $\varepsilon{=}0$ — no bleed to probe bad keys; exploration is via $g(\Delta t)$ only.

---

### 8. Final selection rate

**Normal key**:

$$
R = R_0 + \rho_{\min} \cdot g' \cdot (1 - R_0)
$$

**Hard-fail key (probe-only)**:

$$
R = \rho_{\min} \cdot g' \cdot (1 - R_0) \quad (R{=}0 \text{ when } g'{=}0)
$$

When gate closed ($g'{=}0$) in a multi-key pool: normal keys keep $R{=}R_0$; probe-only keys get $R{=}0$.

$$
W = \text{base} \cdot R \cdot 1000
$$

$\rho_{\min} = 0.02$ — probe floor when gate opens.

---

### Code & YAML mapping

| Symbol | Code / YAML | Default | Role |
|--------|-------------|---------|------|
| $\pi$ | `priorSuccessRate` | 1.0 | Optimistic shrinkage prior |
| $a, b$ | `KeyBetaPriorA/B` | 2, 1 | Beta posterior |
| $w_f$ | `KeyFailureSoftWeight` | 0.3 | Soft failure weight |
| $\tau$ | `KeyConfidenceTau` | 2.0 | Confidence ramp |
| $T_0$ | `KeyProbeSec` | 30s | Probe interval base |
| $\phi$ | `probeFailureScale` | 5.0 | Backoff scale |
| $\rho_{\min}$ | `KeyMinProbeRate` | 0.02 | Probe floor |
| $k$ | `KeyEpsilonScale` | 4 | $\varepsilon$ scale |
| halfLife | `KeyRecoverySec` | 300s | Outcome decay |

`KeyProbeSec = 0` disables time gating (tests).

---

### Behavior summary

| Situation | Effective $R$ |
|-----------|----------------|
| Unexplored key | $\approx 1.0$ |
| QPS throttle (~32% success) | $\approx \rho$ after $\alpha$ rises |
| Dead key just probed | $\approx 0$ (gate closed, probe-only) |
| Dead key after $T(F_b)$ | $\approx \rho_{\min}$ |
| Lone key pool | Never fully zero ($\varepsilon$) |

---

### Route() per-request retry

```text
for each untried key:
    result = amap(key)
    if ok → RecordSuccess(key); return
    else  → RecordFailure(key); continue
return fallback
```

---

## Simulation & calibration

```bash
go run ./scripts/simulate_key_pool.go          # offline; default QPS fail from last live run
go run ./scripts/simulate_key_pool.go -live     # probe good key, then simulate
bash scripts/verify_amap_keys.sh                # key health check
```

Production defaults tuned from live calibration (good-key QPS fail ≈ 68%, $\tau{=}2.0$).

---

## Related reading

- [Architecture](./architecture.md) — provider layer
- [Configuration](./configuration.md) — scheduler & probe settings
