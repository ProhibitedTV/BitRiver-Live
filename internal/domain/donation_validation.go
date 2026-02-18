package domain

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// minDonationAddressLength defines the smallest acceptable donation address length.
	minDonationAddressLength = 4
	// MaxDonationAddressLength defines the largest acceptable donation address length.
	MaxDonationAddressLength = 256
)

// NormalizeDonationAddress trims and validates a donation address, returning the sanitized value.
func NormalizeDonationAddress(addr CryptoAddress) (CryptoAddress, error) {
	currency := strings.ToUpper(strings.TrimSpace(addr.Currency))
	if currency == "" {
		return CryptoAddress{}, fmt.Errorf("donation currency is required")
	}
	for _, r := range currency {
		if r < 'A' || r > 'Z' {
			return CryptoAddress{}, fmt.Errorf("donation currency must contain only uppercase letters")
		}
	}
	address := strings.TrimSpace(addr.Address)
	if address == "" {
		return CryptoAddress{}, fmt.Errorf("donation address is required")
	}
	length := utf8.RuneCountInString(address)
	if length < minDonationAddressLength {
		return CryptoAddress{}, fmt.Errorf("donation address must be at least %d characters", minDonationAddressLength)
	}
	if length > MaxDonationAddressLength {
		return CryptoAddress{}, fmt.Errorf("donation address cannot exceed %d characters", MaxDonationAddressLength)
	}
	for _, r := range address {
		if unicode.IsSpace(r) || !unicode.IsPrint(r) {
			return CryptoAddress{}, fmt.Errorf("donation address contains invalid characters")
		}
	}
	note := strings.TrimSpace(addr.Note)
	return CryptoAddress{Currency: currency, Address: address, Note: note}, nil
}
