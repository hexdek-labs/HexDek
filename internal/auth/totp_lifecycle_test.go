package auth

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// totp_lifecycle_test.go — DB-backed enrollment lifecycle. Reuses
// openTestDB + seedDevice from session_lifecycle_r60_test.go.

func codeForDevice(t *testing.T, d *sql.DB, deviceID string) string {
	t.Helper()
	// Pull the device's stored secret and generate the "right now" code.
	var encoded string
	if err := d.QueryRowContext(context.Background(),
		`SELECT secret FROM device_totp WHERE device_id = ?`, deviceID).
		Scan(&encoded); err != nil {
		t.Fatalf("read secret: %v", err)
	}
	raw, err := DecodeSecret(encoded)
	if err != nil {
		t.Fatalf("decode secret: %v", err)
	}
	return GenerateCode(raw, time.Now().Unix())
}

// --- EnrollTOTP ----------------------------------------------------------

func TestEnrollTOTP_CreatesPendingRow(t *testing.T) {
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	secret, err := EnrollTOTP(context.Background(), d, alice)
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}
	if secret == "" {
		t.Error("returned secret is empty")
	}
	st, err := GetTOTPStatus(context.Background(), d, alice)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st == nil {
		t.Fatal("status nil after enroll")
	}
	if st.Status != "pending" {
		t.Errorf("status = %q, want pending", st.Status)
	}
	if st.ConfirmedAt != 0 {
		t.Errorf("confirmed_at = %d, want 0 before confirm", st.ConfirmedAt)
	}
}

func TestEnrollTOTP_EmptyDeviceErrors(t *testing.T) {
	d := openTestDB(t)
	if _, err := EnrollTOTP(context.Background(), d, ""); err == nil {
		t.Error("empty deviceID should error")
	}
}

func TestEnrollTOTP_ReEnrollReplacesSecretAndResetsToPending(t *testing.T) {
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")

	// First enroll → confirm so we have an active row.
	first, _ := EnrollTOTP(context.Background(), d, alice)
	code := codeForDevice(t, d, alice)
	if err := ConfirmTOTP(context.Background(), d, alice, code); err != nil {
		t.Fatalf("confirm: %v", err)
	}

	// Re-enroll: must replace the secret and roll status back to pending.
	second, err := EnrollTOTP(context.Background(), d, alice)
	if err != nil {
		t.Fatalf("re-enroll: %v", err)
	}
	if second == first {
		t.Error("re-enroll returned the same secret (should be regenerated)")
	}
	st, _ := GetTOTPStatus(context.Background(), d, alice)
	if st == nil || st.Status != "pending" {
		t.Errorf("after re-enroll: want pending, got %+v", st)
	}
	if st.ConfirmedAt != 0 {
		t.Errorf("after re-enroll: confirmed_at = %d, want 0", st.ConfirmedAt)
	}
}

// --- ConfirmTOTP ---------------------------------------------------------

func TestConfirmTOTP_ValidCodeActivates(t *testing.T) {
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	if _, err := EnrollTOTP(context.Background(), d, alice); err != nil {
		t.Fatal(err)
	}
	if err := ConfirmTOTP(context.Background(), d, alice, codeForDevice(t, d, alice)); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	st, _ := GetTOTPStatus(context.Background(), d, alice)
	if st.Status != "active" {
		t.Errorf("status = %q, want active", st.Status)
	}
	if st.ConfirmedAt == 0 {
		t.Error("confirmed_at should be non-zero after activate")
	}
}

func TestConfirmTOTP_WrongCodeRejected(t *testing.T) {
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	_, _ = EnrollTOTP(context.Background(), d, alice)
	if err := ConfirmTOTP(context.Background(), d, alice, "000000"); err != ErrTOTPInvalid {
		// 1-in-10^6 chance of false positive; bump to another wrong code.
		if err == nil {
			err = ConfirmTOTP(context.Background(), d, alice, "999999")
		}
		if err != ErrTOTPInvalid {
			t.Errorf("wrong code: want ErrTOTPInvalid, got %v", err)
		}
	}
	st, _ := GetTOTPStatus(context.Background(), d, alice)
	if st.Status != "pending" {
		t.Errorf("after failed confirm: status = %q, want pending (must stay unconfirmed)", st.Status)
	}
}

func TestConfirmTOTP_NoEnrollmentRejected(t *testing.T) {
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	// No EnrollTOTP call.
	if err := ConfirmTOTP(context.Background(), d, alice, "123456"); err != ErrTOTPInvalid {
		t.Errorf("confirm without enroll: want ErrTOTPInvalid (collapsed with wrong-code), got %v", err)
	}
}

func TestConfirmTOTP_DoubleConfirmReturnsAlreadyActive(t *testing.T) {
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	_, _ = EnrollTOTP(context.Background(), d, alice)
	_ = ConfirmTOTP(context.Background(), d, alice, codeForDevice(t, d, alice))

	// Second confirm — even with a correct code — must surface
	// ErrTOTPAlreadyActive so a racing frontend retry can't replay.
	if err := ConfirmTOTP(context.Background(), d, alice, codeForDevice(t, d, alice)); err != ErrTOTPAlreadyActive {
		t.Errorf("double confirm: want ErrTOTPAlreadyActive, got %v", err)
	}
}

// --- VerifyTOTP ----------------------------------------------------------

func TestVerifyTOTP_AcceptsActiveEnrollment(t *testing.T) {
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	_, _ = EnrollTOTP(context.Background(), d, alice)
	_ = ConfirmTOTP(context.Background(), d, alice, codeForDevice(t, d, alice))
	// Fresh code now that it's active.
	if err := VerifyTOTP(context.Background(), d, alice, codeForDevice(t, d, alice)); err != nil {
		t.Errorf("verify post-confirm: %v", err)
	}
}

func TestVerifyTOTP_RejectsPendingEnrollment(t *testing.T) {
	// A pending (not-yet-confirmed) enrollment must NOT validate
	// production logins — only Confirm should be able to verify
	// against it. Otherwise the user is "logged in with 2FA" using
	// a secret they may have typed a QR scan wrong on.
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	_, _ = EnrollTOTP(context.Background(), d, alice)
	if err := VerifyTOTP(context.Background(), d, alice, codeForDevice(t, d, alice)); err != ErrTOTPNotEnrolled {
		t.Errorf("verify against pending: want ErrTOTPNotEnrolled, got %v", err)
	}
}

func TestVerifyTOTP_RejectsNoEnrollment(t *testing.T) {
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	if err := VerifyTOTP(context.Background(), d, alice, "123456"); err != ErrTOTPNotEnrolled {
		t.Errorf("verify with no row: want ErrTOTPNotEnrolled, got %v", err)
	}
}

func TestVerifyTOTP_WrongCodeOnActiveRejectsAsInvalidNotNotEnrolled(t *testing.T) {
	// The two error classes carry distinct INFO to the server; the
	// caller is responsible for collapsing them on the wire. Pin
	// that the lifecycle layer surfaces them separately.
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	_, _ = EnrollTOTP(context.Background(), d, alice)
	_ = ConfirmTOTP(context.Background(), d, alice, codeForDevice(t, d, alice))
	if err := VerifyTOTP(context.Background(), d, alice, "000000"); err != ErrTOTPInvalid {
		// 1-in-10^6 false positive; try the other side.
		if err == nil {
			err = VerifyTOTP(context.Background(), d, alice, "999999")
		}
		if err != ErrTOTPInvalid {
			t.Errorf("active + wrong code: want ErrTOTPInvalid, got %v", err)
		}
	}
}

func TestVerifyTOTP_EmptyDeviceTreatedAsNotEnrolled(t *testing.T) {
	d := openTestDB(t)
	if err := VerifyTOTP(context.Background(), d, "", "123456"); err != ErrTOTPNotEnrolled {
		t.Errorf("empty deviceID: want ErrTOTPNotEnrolled, got %v", err)
	}
}

// --- GetTOTPStatus -------------------------------------------------------

func TestGetTOTPStatus_NoRowReturnsNilNoError(t *testing.T) {
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	st, err := GetTOTPStatus(context.Background(), d, alice)
	if err != nil {
		t.Errorf("status with no row: unexpected err %v", err)
	}
	if st != nil {
		t.Errorf("status with no row: want nil, got %+v", st)
	}
}

func TestGetTOTPStatus_EmptyDeviceErrors(t *testing.T) {
	d := openTestDB(t)
	if _, err := GetTOTPStatus(context.Background(), d, ""); err == nil {
		t.Error("empty deviceID should error")
	}
}

// --- DisableTOTP ---------------------------------------------------------

func TestDisableTOTP_RemovesActiveEnrollment(t *testing.T) {
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	_, _ = EnrollTOTP(context.Background(), d, alice)
	_ = ConfirmTOTP(context.Background(), d, alice, codeForDevice(t, d, alice))

	// Capture a code BEFORE disabling so the post-disable VerifyTOTP
	// call has something to submit — after disable the secret row is
	// gone and codeForDevice can't read it.
	staleCode := codeForDevice(t, d, alice)

	n, err := DisableTOTP(context.Background(), d, alice)
	if err != nil {
		t.Fatalf("disable: %v", err)
	}
	if n != 1 {
		t.Errorf("removed = %d, want 1", n)
	}
	st, _ := GetTOTPStatus(context.Background(), d, alice)
	if st != nil {
		t.Errorf("after disable: want nil status, got %+v", st)
	}
	if err := VerifyTOTP(context.Background(), d, alice, staleCode); err != ErrTOTPNotEnrolled {
		t.Errorf("verify after disable: want ErrTOTPNotEnrolled, got %v", err)
	}
}

func TestDisableTOTP_NoRowIsIdempotentNoOp(t *testing.T) {
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	n, err := DisableTOTP(context.Background(), d, alice)
	if err != nil {
		t.Errorf("disable on empty: %v", err)
	}
	if n != 0 {
		t.Errorf("removed = %d, want 0 (idempotent on empty)", n)
	}
}

func TestDisableTOTP_EmptyDeviceErrors(t *testing.T) {
	d := openTestDB(t)
	if _, err := DisableTOTP(context.Background(), d, ""); err == nil {
		t.Error("empty deviceID should error so a typo can't mass-disable")
	}
}

// --- End-to-end happy path ----------------------------------------------

func TestTOTP_LifecycleEndToEnd(t *testing.T) {
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")

	// 1. Initially no enrollment.
	if st, _ := GetTOTPStatus(context.Background(), d, alice); st != nil {
		t.Fatalf("baseline: want nil status, got %+v", st)
	}

	// 2. Enroll. Status = pending, verify rejects.
	if _, err := EnrollTOTP(context.Background(), d, alice); err != nil {
		t.Fatal(err)
	}
	if st, _ := GetTOTPStatus(context.Background(), d, alice); st == nil || st.Status != "pending" {
		t.Fatalf("after enroll: want pending status, got %+v", st)
	}
	if err := VerifyTOTP(context.Background(), d, alice, codeForDevice(t, d, alice)); err != ErrTOTPNotEnrolled {
		t.Fatalf("verify against pending: want ErrTOTPNotEnrolled, got %v", err)
	}

	// 3. Confirm with the right code. Status = active.
	if err := ConfirmTOTP(context.Background(), d, alice, codeForDevice(t, d, alice)); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if st, _ := GetTOTPStatus(context.Background(), d, alice); st == nil || st.Status != "active" {
		t.Fatalf("after confirm: want active status, got %+v", st)
	}

	// 4. Verify accepts a current code.
	if err := VerifyTOTP(context.Background(), d, alice, codeForDevice(t, d, alice)); err != nil {
		t.Fatalf("verify post-confirm: %v", err)
	}

	// 5. Disable. Status gone, verify rejects with ErrTOTPNotEnrolled
	//    regardless of what code is submitted (no row → no secret →
	//    not-enrolled wins over invalid).
	if _, err := DisableTOTP(context.Background(), d, alice); err != nil {
		t.Fatal(err)
	}
	if st, _ := GetTOTPStatus(context.Background(), d, alice); st != nil {
		t.Fatalf("post-disable: want nil status, got %+v", st)
	}
	if err := VerifyTOTP(context.Background(), d, alice, "000000"); err != ErrTOTPNotEnrolled {
		t.Errorf("verify post-disable: want ErrTOTPNotEnrolled, got %v", err)
	}
}

// --- Multi-device isolation ---------------------------------------------

func TestTOTP_MultiDeviceIndependent(t *testing.T) {
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	bob := seedDevice(t, d, "device_bob", "Bob")

	// Alice enrolls + confirms; Bob doesn't.
	_, _ = EnrollTOTP(context.Background(), d, alice)
	_ = ConfirmTOTP(context.Background(), d, alice, codeForDevice(t, d, alice))

	// Bob has no enrollment → ErrTOTPNotEnrolled.
	if err := VerifyTOTP(context.Background(), d, bob, codeForDevice(t, d, alice)); err != ErrTOTPNotEnrolled {
		t.Errorf("bob (no enrollment): want ErrTOTPNotEnrolled, got %v", err)
	}

	// Alice's secret MUST NOT verify against bob's deviceID even if
	// the code itself is mathematically valid for alice — the
	// device_id key gates the lookup.

	// Disabling alice doesn't affect any future bob enrollment.
	_, _ = DisableTOTP(context.Background(), d, alice)
	if _, err := EnrollTOTP(context.Background(), d, bob); err != nil {
		t.Fatalf("bob's enroll after alice's disable: %v", err)
	}
}
