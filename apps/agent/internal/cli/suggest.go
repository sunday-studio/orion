package cli

// closestWord returns the candidate nearest to word by edit distance, or ""
// when nothing is close enough to be a likely typo.
func closestWord(word string, candidates []string) string {
	best, bestD := "", 3
	for _, c := range candidates {
		if d := wordDistance(word, c); d < bestD {
			best, bestD = c, d
		}
	}
	return best
}

func wordDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}
