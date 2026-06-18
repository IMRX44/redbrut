package input

// SprayMode controls job generation order.
// Spray: one password against all user/target combos before moving to next password.
// Credential: all passwords for each user/target pair.
type SprayMode bool

const (
	ModeCredential SprayMode = false
	ModeSpray      SprayMode = true
)

// GenerateCombos streams jobs via channel — never holds all combos in RAM.
// Spray mode: password → target → user  (minimizes per-IP attempts per time window)
// Credential mode: target → user → password
func GenerateCombos(targets []Target, users, passwords []string, spray SprayMode) <-chan Job {
	ch := make(chan Job, 50000)
	go func() {
		defer close(ch)
		if spray {
			for _, pass := range passwords {
				for _, target := range targets {
					for _, user := range users {
						ch <- Job{Target: target, Username: user, Password: pass}
					}
				}
			}
		} else {
			for _, target := range targets {
				for _, user := range users {
					for _, pass := range passwords {
						ch <- Job{Target: target, Username: user, Password: pass}
					}
				}
			}
		}
	}()
	return ch
}
