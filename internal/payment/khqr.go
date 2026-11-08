package payment

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type KHQRConfig struct {
	AccountID, MerchantName, MerchantCity, Currency string
	TTL                                             time.Duration
}
type QRData struct {
	Payload   string
	MD5       string
	ExpiresAt time.Time
}

func tlv(tag, value string) string { return fmt.Sprintf("%s%02d%s", tag, len([]byte(value)), value) }
func crc16(data string) uint16 {
	crc := uint16(0xffff)
	for _, b := range []byte(data) {
		crc ^= uint16(b) << 8
		for range 8 {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}
func GenerateKHQR(config KHQRConfig, amount float64, billNumber string, now time.Time) (QRData, error) {
	if config.AccountID == "" {
		return QRData{}, errors.New("BAKONG_ACCOUNT_ID is not configured")
	}
	if len([]byte(config.AccountID)) > 32 || len([]byte(config.MerchantName)) > 25 || len([]byte(config.MerchantCity)) > 15 {
		return QRData{}, errors.New("Bakong merchant fields exceed KHQR limits")
	}
	if amount <= 0 {
		return QRData{}, errors.New("payment amount must be positive")
	}
	if len([]byte(billNumber)) == 0 || len([]byte(billNumber)) > 25 {
		return QRData{}, errors.New("bill number must be 1-25 bytes")
	}
	currency := strings.ToUpper(config.Currency)
	currencyCode := "840"
	amountText := strconv.FormatFloat(amount, 'f', 2, 64)
	if currency == "KHR" {
		currencyCode = "116"
		amountText = strconv.FormatFloat(amount, 'f', 0, 64)
	} else if currency != "USD" {
		return QRData{}, errors.New("currency must be USD or KHR")
	}
	if config.TTL <= 0 || config.TTL > 10*time.Minute {
		return QRData{}, errors.New("KHQR expiry must be between 1 second and 10 minutes")
	}
	expires := now.UTC().Add(config.TTL)
	merchantAccount := tlv("00", config.AccountID)
	additional := tlv("01", billNumber)
	timestamps := tlv("00", strconv.FormatInt(now.UTC().UnixMilli(), 10)) + tlv("01", strconv.FormatInt(expires.UnixMilli(), 10))
	payload := tlv("00", "01") + tlv("01", "12") + tlv("29", merchantAccount) + tlv("52", "5999") + tlv("53", currencyCode) + tlv("54", amountText) + tlv("58", "KH") + tlv("59", config.MerchantName) + tlv("60", config.MerchantCity) + tlv("62", additional) + tlv("99", timestamps) + "6304"
	payload += fmt.Sprintf("%04X", crc16(payload))
	sum := md5.Sum([]byte(payload))
	return QRData{Payload: payload, MD5: hex.EncodeToString(sum[:]), ExpiresAt: expires}, nil
}
