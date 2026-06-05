package validator

import "testing"

type sampleCommand struct {
	Email string `json:"email" validate:"required"`
	Name  string `json:"name"`
}

func TestValidateStruct_WhenRequiredFieldMissing_ReturnsValidationErrors(t *testing.T) {
	err := ValidateStruct(sampleCommand{})
	errs, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("expected ValidationErrors, got %T", err)
	}
	if len(errs) != 1 || errs[0].Field != "email" {
		t.Fatalf("unexpected errors: %#v", errs)
	}
}

func TestValidateStruct_WhenRequiredFieldPresent_ReturnsNil(t *testing.T) {
	if err := ValidateStruct(sampleCommand{Email: "a@example.com"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
