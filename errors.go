package main

import (
	"errors"
	"fmt"
)

type AppError struct {
	Code       string `json:"code"`
	Summary    string `json:"summary"`
	Suggestion string `json:"suggestion,omitempty"`
	Cause      error  `json:"-"`
}

func (e *AppError) Error() string {
	if e.Suggestion == "" {
		return e.Summary
	}
	return e.Summary + "\nSuggestion: " + e.Suggestion
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

func appError(code, summary, suggestion string, cause error) *AppError {
	return &AppError{Code: code, Summary: summary, Suggestion: suggestion, Cause: cause}
}

func appErrorFields(err error) (string, string) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code, appErr.Suggestion
	}
	return "", ""
}

func appErrorSummary(err error) string {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Summary
	}
	return err.Error()
}

func wrapProbeError(input string, err error) error {
	return appError(
		"invalid_media",
		fmt.Sprintf("Could not read %s as a valid media file.", input),
		"Confirm the file is a playable video and try opening it with ffprobe or your NLE. If it is still copying from a card, wait for the copy to finish.",
		err,
	)
}
