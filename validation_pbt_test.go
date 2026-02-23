package kruda

// Validates: Requirements 4.2
//
// Property 1: For any non-empty string s, validateEmail(s, "") agrees with net/mail.ParseAddress(s).
// Property 2: For any non-empty string s, validateURL(s, "") agrees with net/url.Parse (scheme + host check).
// Property 3: For any int n and int p, validateMin(n, strconv.Itoa(p)) iff n >= p.

import (
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"testing/quick"
)

// TestPropertyValidateEmailAgreesWithNetMail checks that for any non-empty string,
// validateEmail agrees with net/mail.ParseAddress.
func TestPropertyValidateEmailAgreesWithNetMail(t *testing.T) {
	f := func(s string) bool {
		if s == "" {
			// Both should return false for empty strings.
			return validateEmail(s, "") == false
		}
		_, err := mail.ParseAddress(s)
		expected := err == nil
		got := validateEmail(s, "")
		return got == expected
	}

	cfg := &quick.Config{MaxCount: 5000}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("validateEmail disagrees with net/mail.ParseAddress: %v", err)
	}
}

// TestPropertyValidateURLAgreesWithNetURL checks that for any non-empty string,
// validateURL agrees with net/url.Parse checking scheme != "" && host != "".
func TestPropertyValidateURLAgreesWithNetURL(t *testing.T) {
	f := func(s string) bool {
		if s == "" {
			// validateURL returns false for empty strings.
			return validateURL(s, "") == false
		}
		u, err := url.Parse(s)
		expected := err == nil && u.Scheme != "" && u.Host != ""
		got := validateURL(s, "")
		return got == expected
	}

	cfg := &quick.Config{MaxCount: 5000}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("validateURL disagrees with net/url.Parse: %v", err)
	}
}

// TestPropertyValidateMinIntComparison checks that for any int n and int p,
// validateMin(n, strconv.Itoa(p)) returns true iff n >= p.
func TestPropertyValidateMinIntComparison(t *testing.T) {
	f := func(n, p int) bool {
		param := strconv.Itoa(p)
		got := validateMin(n, param)
		expected := n >= p
		return got == expected
	}

	cfg := &quick.Config{MaxCount: 5000}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("validateMin disagrees with n >= p: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Feature: phase2b-extensions, Property 6: File Upload max_size Validation
// For any FileUpload with a given Size and any valid size limit,
// validateMaxSize passes iff FileUpload.Size <= parsedLimit
// Validates: R5.1, R5.7
// ---------------------------------------------------------------------------

func TestPropertyValidateMaxSizeAgreesWithParseSize(t *testing.T) {
	// We test with KB, MB, GB suffixes and a range of numeric values.
	// For each generated (fileSize, limitNum, suffix), we compute the expected
	// limit in bytes via parseSize and check validateMaxSize agrees.
	type input struct {
		FileSize int64
		LimitNum uint16 // 0-65535, keeps parseSize in safe int range
		Suffix   uint8  // 0=KB, 1=MB, 2=GB
	}

	f := func(in input) bool {
		// Clamp to positive file size
		if in.FileSize < 0 {
			in.FileSize = -in.FileSize
		}

		// Pick suffix
		suffixes := []string{"kb", "mb", "gb"}
		suffix := suffixes[in.Suffix%3]

		// Avoid zero limit — parseSize("0kb") = 0, which is valid but trivial
		limitNum := in.LimitNum
		if limitNum == 0 {
			limitNum = 1
		}

		param := strconv.Itoa(int(limitNum)) + suffix
		maxBytes, err := parseSize(param)
		if err != nil {
			// parseSize should not fail for valid numeric+suffix
			t.Errorf("parseSize(%q) failed: %v", param, err)
			return false
		}

		fu := &FileUpload{Size: in.FileSize}
		got := validateMaxSize(fu, param)
		expected := in.FileSize <= maxBytes
		return got == expected
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("validateMaxSize disagrees with FileUpload.Size <= parsedLimit: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Feature: phase2b-extensions, Property 7: File Upload mime Validation
// For any FileUpload with a given ContentType and any MIME pattern,
// validateMime passes iff ContentType matches the pattern
// Validates: R5.2, R5.3
// ---------------------------------------------------------------------------

func TestPropertyValidateMimeMatchesPattern(t *testing.T) {
	// Generate random MIME types and patterns, verify validateMime agrees
	// with our reference implementation.
	type input struct {
		TypePart    uint8 // index into type list
		SubTypePart uint8 // index into subtype list
		Wildcard    bool  // if true, pattern uses type/*
		MatchType   bool  // if true, pattern type matches file type
	}

	types := []string{"image", "text", "application", "audio", "video"}
	subtypes := []string{"png", "jpeg", "plain", "html", "json", "pdf", "mp3", "mp4", "xml", "gif"}

	f := func(in input) bool {
		fileType := types[in.TypePart%uint8(len(types))]
		fileSub := subtypes[in.SubTypePart%uint8(len(subtypes))]
		contentType := fileType + "/" + fileSub

		fu := &FileUpload{ContentType: contentType}

		var pattern string
		var expected bool

		if in.Wildcard {
			if in.MatchType {
				// Pattern matches the file's type
				pattern = fileType + "/*"
				expected = true
			} else {
				// Pick a different type for the pattern
				otherType := types[(in.TypePart+1)%uint8(len(types))]
				pattern = otherType + "/*"
				// Expected: true only if otherType == fileType (won't happen since we offset by 1 with >1 types)
				expected = strings.HasPrefix(contentType, otherType+"/")
			}
		} else {
			if in.MatchType {
				// Exact match
				pattern = contentType
				expected = true
			} else {
				// Different subtype
				otherSub := subtypes[(in.SubTypePart+1)%uint8(len(subtypes))]
				pattern = fileType + "/" + otherSub
				expected = contentType == pattern
			}
		}

		got := validateMime(fu, pattern)
		return got == expected
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("validateMime disagrees with expected pattern matching: %v", err)
	}
}
