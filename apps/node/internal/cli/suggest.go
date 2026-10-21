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

// wordDistance is the edit distance where swapping two neighbouring letters
// counts as one edit, because that is how most typos happen (statsu, not status).
func wordDistance(a, b string) int {
	d := make([][]int, len(a)+1)
	for i := range d {
		d[i] = make([]int, len(b)+1)
		d[i][0] = i
	}
	for j := 0; j <= len(b); j++ {
		d[0][j] = j
	}
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			d[i][j] = min(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				d[i][j] = min(d[i][j], d[i-2][j-2]+1)
			}
		}
	}
	return d[len(a)][len(b)]
}
