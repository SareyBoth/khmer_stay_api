package platform

import "testing"

func TestBookingTransitionRequiresVerifiedPayment(t *testing.T) {
	if bookingTransitionAllowed("New request", "Confirmed", "Pending", "owner") {
		t.Fatal("unpaid booking was confirmed")
	}
	if !bookingTransitionAllowed("New request", "Confirmed", "Paid", "owner") {
		t.Fatal("paid booking could not be confirmed")
	}
	if bookingTransitionAllowed("Confirmed", "Completed", "Paid", "owner") {
		t.Fatal("booking skipped check-in")
	}
	if !bookingTransitionAllowed("Checked in", "Completed", "Paid", "owner") {
		t.Fatal("checked-in booking could not complete")
	}
	if bookingTransitionAllowed("Confirmed", "Cancelled", "Paid", "owner") {
		t.Fatal("owner performed admin cancellation")
	}
}
