package arccover

// ComputeLowerBound returns max(LB0, LB1, LB2).
func ComputeLowerBound(n int, required []Arc, maxLegs int) int {
	m := len(required)
	if m == 0 || maxLegs <= 0 {
		return 0
	}
	tau := MinimumTrailCount(n, required)
	lb0 := ceilDiv(m, maxLegs)
	lb1 := ceilDiv(m+tau, maxLegs+1)
	maxFragmentsPerRoute := (maxLegs + 1) / 2
	if maxFragmentsPerRoute < 1 {
		maxFragmentsPerRoute = 1
	}
	lb2 := ceilDiv(tau, maxFragmentsPerRoute)
	return maxInt(lb0, maxInt(lb1, lb2))
}
