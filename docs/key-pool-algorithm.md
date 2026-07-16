# Adaptive Decaying Confidence Scheduler

> A stateless adaptive scheduler that models resource reliability as a **decaying posterior confidence**, balancing exploitation and recovery through continuous feedback — no discrete state machine.

Implementation: [`internal/loadbalance/loadbalance.go`](../internal/loadbalance/loadbalance.go)

---

## Design philosophy

| Principle | Meaning |
|-----------|---------|
| **Continuous state** | `S`, `F_soft`, `F_hard`, `Δt`, `N` replace `healthy/degraded/disabled` |
| **Outcome-based** | Any error counts the same at record time; soft/hard split uses **history**, not infocode |
| **Restless bandit** | Arms decay, backoff, and reprobe over time — not a static bandit |
| **No FSM** | All transitions are implicit in one formula |

---

## 1. Outcome decay

Historical counters decay exponentially (half-life = `KeyRecoverySec`):

$$
S' = S \cdot e^{-\lambda \Delta t}, \quad
F_{\text{soft}}' = F_{\text{soft}} \cdot e^{-\lambda \Delta t}, \quad
F_{\text{hard}}' = F_{\text{hard}} \cdot e^{-\lambda \Delta t}
$$

$$
\lambda = \frac{\ln 2}{\text{KeyRecoverySec}}
$$

---

## 2. Failure decomposition (implicit, not infocode)

On `RecordFailure`:

- If $S > 0$ (has recent success): increment $F_{\text{soft}}$ — e.g. QPS throttle mixed with OK
- Else: increment $F_{\text{hard}}$ — likely dead / IP / recycled key

Keys with $S{=}0$ and $F_{\text{hard}} \ge 1$ enter **probe-only** mode: $R \approx \rho_{\min} \cdot g'$ when the gate opens; $R{=}0$ while the gate is closed. Below one effective hard failure, the key is treated as unexplored again.

$$
F = F_{\text{soft}} + F_{\text{hard}}
$$

Backoff uses weighted failures:

$$
F_b = F_{\text{hard}} + w_f \cdot F_{\text{soft}}, \quad w_f = 0.3
$$

---

## 3. Posterior success rate (Beta smoothing)

Optimistic prior $\text{Beta}(a{=}2,\, b{=}1)$ — unexplored keys start at $\rho \approx 0.67$, not $0.5$:

$$
\rho = \frac{S + a}{S + F + a + b}
$$

Constants: $a=2$, $b=1$.

---

## 4. Confidence ramp (when to trust history)

Weighted sample mass — failures contribute less to confidence than successes:

$$
\eta = S + w_f \cdot F
$$

$$
\alpha = 1 - e^{-\eta / \tau}, \quad \tau = 2
$$

$\alpha$ controls exploit vs prior $\pi$ only; $\rho$ itself remains asymmetric via Beta.

---

## 5. Shrinkage reward

$$
R_0 = (1-\alpha)\,\pi + \alpha\,\rho, \quad \pi = 1.0
$$

Unexplored ($\eta{=}0$): $\alpha{=}0 \Rightarrow R_0{=}\pi{=}1$ (full weight).

---

## 6. Time-gated probe (backoff)

$$
T(F_b) = T_0 \cdot e^{F_b / \phi}
$$

$$
g(\Delta t) = 1 - e^{-\Delta t / T(F_b)}
$$

Just probed ($\Delta t{=}0$): $g{=}0$. After waiting $\gg T(F_b)$: $g \to 1$.

$T_0 = \texttt{KeyProbeSec}$ (default 30s), $\phi = 5$.

---

## 7. Single-key anti-starvation

Exploration comes from $g(\Delta t)$, not from $\varepsilon$. $\varepsilon$ only prevents $R{=}0$ when $N{=}1$:

$$
\varepsilon(N) = \begin{cases} e^{-N/4} & N \le 1 \\ 0 & N > 1 \end{cases}
$$

$$
g' = g + (1-g)\,\varepsilon(N)
$$

---

## 8. Final selection rate

$$
R = R_0 + \rho_{\min} \cdot g' \cdot (1 - R_0)
$$

Gate closed ($g'{=}0$, multi-key): $R = R_0$ (exploit). Probe term opens on schedule via $g(\Delta t)$.

$$
W = \text{base} \cdot R \cdot 1000
$$

$\rho_{\min} = 0.02$ — probe floor when gate opens.

---

## Parameters

| Symbol | Code constant | Default | Role |
|--------|---------------|---------|------|
| $\pi$ | `priorSuccessRate` | 1.0 | Optimistic shrinkage prior |
| $a, b$ | `betaPriorA/B` | 2, 1 | Beta posterior |
| $w_f$ | `failureSoftWeight` | 0.3 | Soft failure weight |
| $\tau$ | `KeyConfidenceTau` | 2.0 | Confidence ramp speed (configurable) |
| $T_0$ | `KeyProbeSec` | 30s | Probe interval base |
| $\phi$ | `probeFailureScale` | 5.0 | Backoff scale |
| $\rho_{\min}$ | `minProbeRate` | 0.02 | Probe floor |

`KeyProbeSec = 0` disables time gating (tests).

---

## Behavior summary

| Situation | Effective $R$ |
|-----------|----------------|
| Unexplored key | $\approx 1.0$ |
| QPS throttle (~32% success) | $\approx \rho$ after $\alpha$ rises |
| Dead key just probed | $\approx 0$ (gate closed) |
| Dead key after $T(F_b)$ | $\approx \rho_{\min}$ |
| Lone key pool | Never fully zero ($\varepsilon$) |

---

## Route() per-request retry

```text
for each untried key:
    result = amap(key)
    if ok → RecordSuccess(key); return
    else  → RecordFailure(key); continue
return fallback
```

---

## Simulation & evaluation

```bash
go run ./scripts/simulate_key_pool.go          # offline, default QPS fail from last live run
go run ./scripts/simulate_key_pool.go -live     # probe good key, then simulate
bash scripts/verify_amap_keys.sh                # key health check
```

Production defaults in `etc/matrix.yaml` were chosen from live calibration (68% QPS fail on good key, τ=2.0).

---

## Related reading

- [Architecture](./architecture.md) — provider layer
- [Configuration](./configuration.md) — scheduler and probe settings
