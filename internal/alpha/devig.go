package alpha

import "errors"

var (
	ErrInvalidOdds   = errors.New("alpha: odds must be non-zero")
	ErrNoConvergence = errors.New("alpha: Shin bisection did not converge within 50 iterations")
)

const (
	shinEpsilon = 1e-9
	shinMaxIter = 50
)

// americanToImplied converts American odds to an implied probability.
//
//	odds < 0 (favorite): |odds| / (|odds| + 100)
//	odds > 0 (underdog): 100   / (odds   + 100)
func americanToImplied(odds int64) (float64, error) {
	if odds == 0 {
		return 0, ErrInvalidOdds
	}
	if odds < 0 {
		f := float64(-odds)
		return f / (f + 100.0), nil
	}
	return 100.0 / (float64(odds) + 100.0), nil
}

// Devig2Way strips the overround from a two-way market using closed-form
// normalization. Returns fair no-vig probabilities for both outcomes.
func Devig2Way(odds1, odds2 int64) ([2]float64, error) {
	p1, err := americanToImplied(odds1)
	if err != nil {
		return [2]float64{}, err
	}
	p2, err := americanToImplied(odds2)
	if err != nil {
		return [2]float64{}, err
	}
	inv := 1.0 / (p1 + p2)
	return [2]float64{p1 * inv, p2 * inv}, nil
}

// Devig3Way strips the overround from a three-way market using the Shin Method.
// Bisects z ∈ (0,1) until Σ[p_i²/(z+(1-z)·p_i)] = 1 within shinEpsilon.
// Returns fair no-vig probabilities for all three outcomes.
func Devig3Way(odds1, odds2, odds3 int64) ([3]float64, error) {
	p1, err := americanToImplied(odds1)
	if err != nil {
		return [3]float64{}, err
	}
	p2, err := americanToImplied(odds2)
	if err != nil {
		return [3]float64{}, err
	}
	p3, err := americanToImplied(odds3)
	if err != nil {
		return [3]float64{}, err
	}

	zLo, zHi := 0.0, 1.0
	var z, s float64
	converged := false

	for i := 0; i < shinMaxIter; i++ {
		z = (zLo + zHi) * 0.5
		s = shinSum3(z, p1, p2, p3)
		diff := s - 1.0
		if diff < 0 {
			diff = -diff
		}
		if diff < shinEpsilon {
			converged = true
			break
		}
		// S is strictly decreasing in z:
		//   S(z=0) = Σp_i > 1 (vig market)
		//   S(z=1) = Σp_i² < 1
		// S > 1 → z is too low → move lower bound up.
		if s > 1.0 {
			zLo = z
		} else {
			zHi = z
		}
	}

	if !converged {
		return [3]float64{}, ErrNoConvergence
	}

	// Recompute final S at converged z and normalize.
	s = shinSum3(z, p1, p2, p3)
	inv := 1.0 / s
	f1 := (p1 * p1) / (z + (1-z)*p1) * inv
	f2 := (p2 * p2) / (z + (1-z)*p2) * inv
	f3 := (p3 * p3) / (z + (1-z)*p3) * inv

	return [3]float64{f1, f2, f3}, nil
}

// shinSum3 computes Σ[p_i²/(z+(1-z)·p_i)] for three outcomes.
// Inlined for hot-path performance.
func shinSum3(z, p1, p2, p3 float64) float64 {
	z1 := 1.0 - z
	return (p1*p1)/(z+z1*p1) + (p2*p2)/(z+z1*p2) + (p3*p3)/(z+z1*p3)
}
