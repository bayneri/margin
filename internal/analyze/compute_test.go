package analyze

import "testing"

func TestComputeBudget(t *testing.T) {
	allowed, bad, consumed, _ := ComputeBudget(0.999, 0.995)
	if allowed <= 0 {
		t.Fatalf("expected allowedBad > 0")
	}
	if bad <= 0 {
		t.Fatalf("expected bad > 0")
	}
	if consumed <= 0 {
		t.Fatalf("expected consumed > 0")
	}
}

func TestComputeBudgetGoal100(t *testing.T) {
	allowed, _, consumed, _ := ComputeBudget(1.0, 1.0)
	if allowed != 0 {
		t.Fatalf("expected allowedBad 0, got %v", allowed)
	}
	if consumed != 0 {
		t.Fatalf("expected consumed 0, got %v", consumed)
	}
}
