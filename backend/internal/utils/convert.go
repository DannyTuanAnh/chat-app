package utils

import (
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

var (
	matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
	matchAllCap   = regexp.MustCompile("([a-z0-9])([A-Z])")
)

func CamelToSnake(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
}

// NormalizeString function is used to normalize a string by trimming leading and trailing whitespace and converting it to lowercase.
func NormalizeString(text string) string {
	return strings.ToLower(strings.TrimSpace(text))
}

// ConvertMapToSliceWithTransform converts a map to a slice and applies a transformation function to each value in one pass.
// It takes a map of type map[K]V and a transformation function that converts values of type V to type R,
// and returns a slice of type []R containing the transformed values.
// This function is useful for efficiently converting a map to a slice while applying a transformation
// to each value without needing to create an intermediate slice of the original values.
func ConvertMapToSliceWithTransform[K comparable, V any, R any](m map[K]V, transform func(V) R) []R {
	result := make([]R, 0, len(m))
	for _, value := range m {
		result = append(result, transform(value))
	}

	return result
}

func ConvertToPgTypeText(input string) pgtype.Text {
	if strings.TrimSpace(input) == "" {
		return pgtype.Text{
			Valid: false,
		}
	}

	return pgtype.Text{
		String: input,
		Valid:  true,
	}
}

func ConvertToPgTypeTextPtr(input *string) pgtype.Text {
	if input == nil || strings.TrimSpace(*input) == "" {
		return pgtype.Text{
			Valid: false,
		}
	}

	return pgtype.Text{
		String: *input,
		Valid:  true,
	}
}

func ConvertToPgTypeIntPtr(input *int) pgtype.Int4 {
	if input == nil || *input == 0 {
		return pgtype.Int4{
			Valid: false,
		}
	}

	return pgtype.Int4{
		Int32: int32(*input),
		Valid: true,
	}
}

func ConvertToPgTypeDate(input string) pgtype.Date {
	if strings.TrimSpace(input) == "" {
		return pgtype.Date{
			Valid: false,
		}
	}

	t, err := time.Parse("2006-01-02", input)
	if err != nil {
		return pgtype.Date{
			Valid: false,
		}
	}

	return pgtype.Date{
		Time:  t,
		Valid: true,
	}
}

func ConvertToPgTypeTime(input time.Time) pgtype.Time {
	if input.IsZero() {
		return pgtype.Time{
			Valid: false,
		}
	}

	nano := time.Duration(input.Hour())*time.Hour +
		time.Duration(input.Minute())*time.Minute +
		time.Duration(input.Second())*time.Second +
		time.Duration(input.Nanosecond())

	return pgtype.Time{
		Microseconds: nano.Microseconds(),
		Valid:        true,
	}
}

func ConvertToPgTypeDatePtr(input *string) pgtype.Date {
	if input == nil || strings.TrimSpace(*input) == "" {
		return pgtype.Date{
			Valid: false,
		}
	}

	t, err := time.Parse("2006-01-02", *input)
	if err != nil {
		return pgtype.Date{
			Valid: false,
		}
	}

	return pgtype.Date{
		Time:  t,
		Valid: true,
	}
}
