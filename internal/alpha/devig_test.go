package alpha

import (
	"errors"
	"math"
	"testing"
)

const resultTol = 1e-6
const sumTol = 1e-9

// --- Devig2Way ---

func TestDevig2WaySymmetric(t *testing.T) {
	got, err := Devig2Way(-110, -110)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// -110/-110: p=110/210 each, overround=220/210, fair=[0.5, 0.5]
	if math.Abs(got[0]-0.5) > resultTol {
		t.Errorf("f1 = %.10f, want 0.5", got[0])
	}
	if math.Abs(got[1]-0.5) > resultTol {
		t.Errorf("f2 = %.10f, want 0.5", got[1])
	}
	if math.Abs(got[0]+got[1]-1.0) > sumTol {
		t.Errorf("sum = %.15f, want 1.0", got[0]+got[1])
	}
}

func TestDevig2WayLopsided(t *testing.T) {
	got, err := Devig2Way(-200, 170)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// p1=2/3, p2=10/27, V=28/27 → f1=9/14, f2=5/14
	want1 := 9.0 / 14.0
	want2 := 5.0 / 14.0
	if math.Abs(got[0]-want1) > resultTol {
		t.Errorf("f1 = %.10f, want %.10f (9/14)", got[0], want1)
	}
	if math.Abs(got[1]-want2) > resultTol {
		t.Errorf("f2 = %.10f, want %.10f (5/14)", got[1], want2)
	}
	if math.Abs(got[0]+got[1]-1.0) > sumTol {
		t.Errorf("sum = %.15f, want 1.0", got[0]+got[1])
	}
}

func TestDevig2WayTightLine(t *testing.T) {
	got, err := Devig2Way(-105, -115)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(got[0]+got[1]-1.0) > sumTol {
		t.Errorf("sum = %.15f, want 1.0", got[0]+got[1])
	}
	for i, f := range got {
		if f <= 0 || f >= 1 {
			t.Errorf("got[%d] = %f out of (0,1)", i, f)
		}
	}
	// -115 side has higher implied prob than -105
	if got[0] >= got[1] {
		t.Errorf("f1(%.6f) should be < f2(%.6f): -115 is bigger favorite", got[0], got[1])
	}
}

func TestDevig2WayPositiveBothSides(t *testing.T) {
	// Both sides positive odds (high vig underdog market)
	got, err := Devig2Way(150, 120)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(got[0]+got[1]-1.0) > sumTol {
		t.Errorf("sum = %.15f, want 1.0", got[0]+got[1])
	}
	// +120 side has higher implied prob than +150
	if got[0] >= got[1] {
		t.Errorf("f1(%.6f) should be < f2(%.6f): +120 implied > +150 implied", got[0], got[1])
	}
}

func TestDevig2WayZeroFirstOdds(t *testing.T) {
	_, err := Devig2Way(0, -110)
	if !errors.Is(err, ErrInvalidOdds) {
		t.Errorf("expected ErrInvalidOdds, got %v", err)
	}
}

func TestDevig2WayZeroSecondOdds(t *testing.T) {
	_, err := Devig2Way(-110, 0)
	if !errors.Is(err, ErrInvalidOdds) {
		t.Errorf("expected ErrInvalidOdds, got %v", err)
	}
}

// --- Devig3Way ---

func TestDevig3WaySumToOne(t *testing.T) {
	got, err := Devig3Way(-125, 270, 450)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sum := got[0] + got[1] + got[2]
	if math.Abs(sum-1.0) > sumTol {
		t.Errorf("sum = %.15f, want 1.0 ± 1e-9", sum)
	}
}

func TestDevig3WayProbsInRange(t *testing.T) {
	got, err := Devig3Way(-125, 270, 450)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, f := range got {
		if f <= 0 || f >= 1 {
			t.Errorf("got[%d] = %f out of (0,1)", i, f)
		}
	}
}

func TestDevig3WayFavoriteOrder(t *testing.T) {
	// -125 favorite, +270 draw, +450 underdog
	got, err := Devig3Way(-125, 270, 450)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[0] <= got[1] {
		t.Errorf("f_home(%.6f) should be > f_draw(%.6f)", got[0], got[1])
	}
	if got[1] <= got[2] {
		t.Errorf("f_draw(%.6f) should be > f_away(%.6f)", got[1], got[2])
	}
}

func TestDevig3WayNearEqualOdds(t *testing.T) {
	got, err := Devig3Way(-105, -105, -105)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sum := got[0] + got[1] + got[2]
	if math.Abs(sum-1.0) > sumTol {
		t.Errorf("sum = %.15f, want 1.0", sum)
	}
	// Equal odds → each fair prob ≈ 1/3
	for i, f := range got {
		if math.Abs(f-1.0/3.0) > 0.01 {
			t.Errorf("got[%d] = %.6f, want near 1/3", i, f)
		}
	}
}

func TestDevig3WayHighVig(t *testing.T) {
	// High vig market (typical retail sportsbook 3-way)
	got, err := Devig3Way(-110, 240, 380)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sum := got[0] + got[1] + got[2]
	if math.Abs(sum-1.0) > sumTol {
		t.Errorf("sum = %.15f, want 1.0", sum)
	}
	for i, f := range got {
		if f <= 0 || f >= 1 {
			t.Errorf("got[%d] = %f out of (0,1)", i, f)
		}
	}
}

func TestDevig3WayZeroOdds(t *testing.T) {
	_, err := Devig3Way(0, 270, 450)
	if !errors.Is(err, ErrInvalidOdds) {
		t.Errorf("expected ErrInvalidOdds, got %v", err)
	}
	_, err = Devig3Way(-125, 0, 450)
	if !errors.Is(err, ErrInvalidOdds) {
		t.Errorf("expected ErrInvalidOdds for second arg, got %v", err)
	}
	_, err = Devig3Way(-125, 270, 0)
	if !errors.Is(err, ErrInvalidOdds) {
		t.Errorf("expected ErrInvalidOdds for third arg, got %v", err)
	}
}

func TestDevig3WayErrNoConvergenceSentinelExists(t *testing.T) {
	// ErrNoConvergence must be exported and distinct from ErrInvalidOdds.
	if ErrNoConvergence == nil {
		t.Fatal("ErrNoConvergence is nil")
	}
	if errors.Is(ErrNoConvergence, ErrInvalidOdds) {
		t.Fatal("ErrNoConvergence must be distinct from ErrInvalidOdds")
	}
}
