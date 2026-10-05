package cluster

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
)

func TestTokenDurationFor(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		creds   aws.Credentials
		want    time.Duration
		wantErr bool
	}{
		{
			name:  "When credentials do not expire it should request the full duration",
			creds: aws.Credentials{CanExpire: false},
			want:  tokenDuration,
		},
		{
			name:  "When the session has more than 15 minutes left it should request the full duration",
			creds: aws.Credentials{CanExpire: true, Expires: now.Add(time.Hour)},
			want:  tokenDuration,
		},
		{
			name:  "When the session has less than 15 minutes left it should end before the session",
			creds: aws.Credentials{CanExpire: true, Expires: now.Add(10 * time.Minute)},
			want:  10*time.Minute - sessionExpiryMargin,
		},
		{
			name:  "When the remaining time has a fraction of a second it should round down",
			creds: aws.Credentials{CanExpire: true, Expires: now.Add(5*time.Minute + 700*time.Millisecond)},
			want:  5*time.Minute - sessionExpiryMargin,
		},
		{
			name:  "When the session leaves exactly the STS minimum it should request the minimum",
			creds: aws.Credentials{CanExpire: true, Expires: now.Add(minTokenDuration + sessionExpiryMargin)},
			want:  minTokenDuration,
		},
		{
			name:    "When the session leaves less than the STS minimum it should fail",
			creds:   aws.Credentials{CanExpire: true, Expires: now.Add(80 * time.Second)},
			wantErr: true,
		},
		{
			name:    "When the session has already expired it should fail",
			creds:   aws.Credentials{CanExpire: true, Expires: now.Add(-time.Minute)},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tokenDurationFor(now, tt.creds)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %s", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("tokenDurationFor() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestRefreshSkew(t *testing.T) {
	if got := refreshSkew(tokenDuration); got != tokenRefreshSkew {
		t.Errorf("When the token is full length it should refresh %s early, got %s", tokenRefreshSkew, got)
	}
	if got := refreshSkew(minTokenDuration); got != 15*time.Second {
		t.Errorf("When the token is 60s it should refresh 15s early, got %s", got)
	}
}
