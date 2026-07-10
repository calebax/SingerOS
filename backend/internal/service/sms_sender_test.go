package service

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsAliyunSMSRateLimited(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		message string
		want    bool
	}{
		{
			name:    "minute level flow control message",
			message: "触发分钟级流控Permits:1",
			want:    true,
		},
		{
			name: "business limit control code",
			code: "isv.BUSINESS_LIMIT_CONTROL",
			want: true,
		},
		{
			name:    "generic sms failure",
			code:    "isv.INVALID_PARAMETERS",
			message: "参数异常",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isAliyunSMSRateLimited(tt.code, tt.message); got != tt.want {
				t.Fatalf("isAliyunSMSRateLimited(%q, %q) = %v, want %v", tt.code, tt.message, got, tt.want)
			}
		})
	}
}

func TestMapSMSSendError(t *testing.T) {
	rateLimitedErr := fmt.Errorf("%w: %s", errAliyunSMSRateLimited, "触发分钟级流控Permits:1")
	if !errors.Is(mapSMSSendError(rateLimitedErr), errAuthPhoneCodeSendTooOften) {
		t.Fatalf("expected phone code send too often error")
	}

	genericErr := fmt.Errorf("aliyun sms failed: %s", "参数异常")
	if !errors.Is(mapSMSSendError(genericErr), errAuthSMSDeliveryFailed) {
		t.Fatalf("expected sms delivery failed error")
	}
}
