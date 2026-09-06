package trakt

import (
	"fmt"
	"net/http"
)

type UnexpectedStatusCodeError struct {
	Got  int
	Want []int
	URL  string
	Body string
}

func (e *UnexpectedStatusCodeError) Error() string {
	return fmt.Sprintf("unexpected status code: got %d, want one of %d (url: %s, body: %s)", e.Got, e.Want, e.URL, e.Body)
}

func NewUnexpectedStatusCodeError(got int, reqURL, body string, want ...int) error {
	return &UnexpectedStatusCodeError{
		Got:  got,
		Want: want,
		URL:  reqURL,
		Body: body,
	}
}

type AccountLimitExceededError struct {
	accountLimit string
}

func (e *AccountLimitExceededError) Error() string {
	return fmt.Sprintf("trakt account limit (%s) exceeded, more info here: https://forums.trakt.tv/t/freemium-experience-more-features-for-all-with-usage-limits/41641", e.accountLimit)
}

func NewAccountLimitExceededError(headers http.Header) error {
	return &AccountLimitExceededError{
		accountLimit: headers.Get("X-Account-Limit"),
	}
}

type ListNotFoundError struct {
	ID string
}

func (e *ListNotFoundError) Error() string {
	return fmt.Sprintf("list with id %s could not be found", e.ID)
}

func NewListNotFoundError(id string) error {
	return &ListNotFoundError{
		ID: id,
	}
}
