package models

import "testing"

const (
	adminHash = "pbkdf2_sha256$1500000$xT0rOS1a20sYdLxorfNVug$IbfYM7jlW/MARo2rYhVAYSMStjGUpPQFCXnapXn2O5o="
	guestHash = "pbkdf2_sha256$1500000$bFB6pZVMgh6Zo7yPv8VvAn$2GrjnMJiy6PKnlJylr53LopAFUmHvhFR3jSkx7rzYM4="
)

func TestVerifyPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		encoded  string
		want     bool
		wantErr  bool
	}{
		{"correct admin", "admin", adminHash, true, false},
		{"correct guest", "guest", guestHash, true, false},
		{"wrong password", "guest", adminHash, false, false},
		{"empty password", "", adminHash, false, false},
		{"unsupported algorithm", "admin", "bcrypt$12$salt$hash", false, true},
		{"malformed hash", "admin", "nonsense", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := verifyPassword(tt.password, tt.encoded)

			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
