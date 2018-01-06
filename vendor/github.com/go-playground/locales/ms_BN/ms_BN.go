package ms_BN

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type ms_BN struct {
	locale                 string
	pluralsCardinal        []locales.PluralRule
	pluralsOrdinal         []locales.PluralRule
	pluralsRange           []locales.PluralRule
	decimal                string
	group                  string
	minus                  string
	percent                string
	perMille               string
	timeSeparator          string
	inifinity              string
	currencies             []string // idx = enum of currency code
	currencyPositivePrefix string
	currencyNegativePrefix string
	monthsAbbreviated      []string
	monthsNarrow           []string
	monthsWide             []string
	daysAbbreviated        []string
	daysNarrow             []string
	daysShort              []string
	daysWide               []string
	periodsAbbreviated     []string
	periodsNarrow          []string
	periodsShort           []string
	periodsWide            []string
	erasAbbreviated        []string
	erasNarrow             []string
	erasWide               []string
	timezones              map[string]string
}

// New returns a new instance of translator for the 'ms_BN' locale
func New() locales.Translator {
	return &ms_BN{
		locale:                 "ms_BN",
		pluralsCardinal:        []locales.PluralRule{6},
		pluralsOrdinal:         []locales.PluralRule{2, 6},
		pluralsRange:           []locales.PluralRule{6},
		decimal:                ",",
		group:                  ".",
		minus:                  "-",
		percent:                "%",
		perMille:               "‰",
		timeSeparator:          ":",
		inifinity:              "∞",
		currencies:             []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "AUD", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "$", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "BRL", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CAD", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CNY", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "EUR", "FIM", "FJD", "FKP", "FRF", "GBP", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HKD", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "ILS", "INR", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "JPY", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "KRW", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MXN", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZD", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "THB", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "TWD", "TZS", "UAH", "UAK", "UGS", "UGX", "USD", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "VND", "VNN", "VUV", "WST", "XAF", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "XCD", "XDR", "XEU", "XFO", "XFU", "XOF", "XPD", "XPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		currencyPositivePrefix: " ",
		currencyNegativePrefix: " ",
		monthsAbbreviated:      []string{"", "Jan", "Feb", "Mac", "Apr", "Mei", "Jun", "Jul", "Ogo", "Sep", "Okt", "Nov", "Dis"},
		monthsNarrow:           []string{"", "J", "F", "M", "A", "M", "J", "J", "O", "S", "O", "N", "D"},
		monthsWide:             []string{"", "Januari", "Februari", "Mac", "April", "Mei", "Jun", "Julai", "Ogos", "September", "Oktober", "November", "Disember"},
		daysAbbreviated:        []string{"Ahd", "Isn", "Sel", "Rab", "Kha", "Jum", "Sab"},
		daysNarrow:             []string{"A", "I", "S", "R", "K", "J", "S"},
		daysShort:              []string{"Ah", "Is", "Se", "Ra", "Kh", "Ju", "Sa"},
		daysWide:               []string{"Ahad", "Isnin", "Selasa", "Rabu", "Khamis", "Jumaat", "Sabtu"},
		periodsAbbreviated:     []string{"PG", "PTG"},
		periodsNarrow:          []string{"a", "p"},
		periodsWide:            []string{"PG", "PTG"},
		erasAbbreviated:        []string{"S.M.", "TM"},
		erasNarrow:             []string{"", ""},
		erasWide:               []string{"S.M.", "TM"},
		timezones:              map[string]string{"NZDT": "Waktu Siang New Zealand", "JST": "Waktu Piawai Jepun", "ARST": "Waktu Musim Panas Argentina", "HKT": "Waktu Piawai Hong Kong", "ACDT": "Waktu Siang Australia Tengah", "WESZ": "Waktu Musim Panas Eropah Barat", "HADT": "Waktu Siang Hawaii-Aleutian", "UYT": "Waktu Piawai Uruguay", "UYST": "Waktu Musim Panas Uruguay", "WIT": "Waktu Indonesia Timur", "MESZ": "Waktu Musim Panas Eropah Tengah", "NZST": "Waktu Piawai New Zealand", "HNNOMX": "Waktu Piawai Barat Laut Mexico", "HENOMX": "Waktu Siang Barat Laut Mexico", "MYT": "Waktu Malaysia", "CLT": "Waktu Piawai Chile", "PST": "Waktu Piawai Pasifik", "HNPM": "Waktu Piawai Saint Pierre dan Miquelon", "LHST": "Waktu Piawai Lord Howe", "HNOG": "Waktu Piawai Greenland Barat", "HNT": "Waktu Piawai Newfoundland", "ACST": "Waktu Piawai Australia Tengah", "ChST": "Waktu Piawai Chamorro", "OESZ": "Waktu Musim Panas Eropah Timur", "CDT": "Waktu Siang Tengah", "AWDT": "Waktu Siang Australia Barat", "AEDT": "Waktu Siang Australia Timur", "WAT": "Waktu Piawai Afrika Barat", "WAST": "Waktu Musim Panas Afrika Barat", "HNEG": "Waktu Piawai Greenland Timur", "BT": "Waktu Bhutan", "TMST": "Waktu Musim Panas Turkmenistan", "AST": "Waktu Piawai Atlantik", "SAST": "Waktu Piawai Afrika Selatan", "HAT": "Waktu Siang Newfoundland", "CAT": "Waktu Afrika Tengah", "∅∅∅": "∅∅∅", "AEST": "Waktu Piawai Timur Australia", "EAT": "Waktu Afrika Timur", "WEZ": "Waktu Piawai Eropah Barat", "HNCU": "Waktu Piawai Cuba", "IST": "Waktu Piawai India", "HAST": "Waktu Piawai Hawaii-Aleutian", "VET": "Waktu Venezuela", "ADT": "Waktu Siang Atlantik", "HEOG": "Waktu Musim Panas Greenland Barat", "AKDT": "Waktu Siang Alaska", "WIB": "Waktu Indonesia Barat", "HEPM": "Waktu Siang Saint Pierre dan Miquelon", "SRT": "Waktu Suriname", "CLST": "Waktu Musim Panas Chile", "COST": "Waktu Musim Panas Colombia", "EST": "Waktu Piawai Timur", "HEPMX": "Waktu Siang Pasifik Mexico", "CHAST": "Waktu Piawai Chatham", "ACWST": "Waktu Piawai Barat Tengah Australia", "OEZ": "Waktu Piawai Eropah Timur", "ART": "Waktu Piawai Argentina", "GYT": "Waktu Guyana", "MST": "MST", "COT": "Waktu Piawai Colombia", "GMT": "Waktu Min Greenwich", "ACWDT": "Waktu Siang Barat Tengah Australia", "CST": "Waktu Piawai Pusat", "EDT": "Waktu Siang Timur", "AKST": "Waktu Piawai Alaska", "ECT": "Waktu Ecuador", "BOT": "Waktu Bolivia", "MEZ": "Waktu Piawai Eropah Tengah", "HNPMX": "Waktu Piawai Pasifik Mexico", "AWST": "Waktu Piawai Australia Barat", "WARST": "Waktu Musim Panas Argentina Barat", "GFT": "Waktu Guyana Perancis", "HKST": "Waktu Musim Panas Hong Kong", "SGT": "Waktu Piawai Singapura", "WITA": "Waktu Indonesia Tengah", "JDT": "Waktu Siang Jepun", "WART": "Waktu Piawai Argentina Barat", "PDT": "Waktu Siang Pasifik", "CHADT": "Waktu Siang Chatham", "MDT": "MDT", "LHDT": "Waktu Siang Lord Howe", "HEEG": "Waktu Musim Panas Greenland Timur", "HECU": "Waktu Siang Cuba", "TMT": "Waktu Piawai Turkmenistan"},
	}
}

// Locale returns the current translators string locale
func (ms *ms_BN) Locale() string {
	return ms.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'ms_BN'
func (ms *ms_BN) PluralsCardinal() []locales.PluralRule {
	return ms.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'ms_BN'
func (ms *ms_BN) PluralsOrdinal() []locales.PluralRule {
	return ms.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'ms_BN'
func (ms *ms_BN) PluralsRange() []locales.PluralRule {
	return ms.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'ms_BN'
func (ms *ms_BN) CardinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'ms_BN'
func (ms *ms_BN) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)

	if n == 1 {
		return locales.PluralRuleOne
	}

	return locales.PluralRuleOther
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'ms_BN'
func (ms *ms_BN) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {
	return locales.PluralRuleOther
}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (ms *ms_BN) MonthAbbreviated(month time.Month) string {
	return ms.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (ms *ms_BN) MonthsAbbreviated() []string {
	return ms.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (ms *ms_BN) MonthNarrow(month time.Month) string {
	return ms.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (ms *ms_BN) MonthsNarrow() []string {
	return ms.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (ms *ms_BN) MonthWide(month time.Month) string {
	return ms.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (ms *ms_BN) MonthsWide() []string {
	return ms.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (ms *ms_BN) WeekdayAbbreviated(weekday time.Weekday) string {
	return ms.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (ms *ms_BN) WeekdaysAbbreviated() []string {
	return ms.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (ms *ms_BN) WeekdayNarrow(weekday time.Weekday) string {
	return ms.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (ms *ms_BN) WeekdaysNarrow() []string {
	return ms.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (ms *ms_BN) WeekdayShort(weekday time.Weekday) string {
	return ms.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (ms *ms_BN) WeekdaysShort() []string {
	return ms.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (ms *ms_BN) WeekdayWide(weekday time.Weekday) string {
	return ms.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (ms *ms_BN) WeekdaysWide() []string {
	return ms.daysWide
}

// Decimal returns the decimal point of number
func (ms *ms_BN) Decimal() string {
	return ms.decimal
}

// Group returns the group of number
func (ms *ms_BN) Group() string {
	return ms.group
}

// Group returns the minus sign of number
func (ms *ms_BN) Minus() string {
	return ms.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'ms_BN' and handles both Whole and Real numbers based on 'v'
func (ms *ms_BN) FmtNumber(num float64, v uint64) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 2 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, ms.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, ms.group[0])
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, ms.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	return string(b)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'ms_BN' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (ms *ms_BN) FmtPercent(num float64, v uint64) string {
	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 3
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, ms.decimal[0])
			continue
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, ms.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	b = append(b, ms.percent...)

	return string(b)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'ms_BN'
func (ms *ms_BN) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := ms.currencies[currency]
	l := len(s) + len(symbol) + 4 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, ms.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, ms.group[0])
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	for j := len(symbol) - 1; j >= 0; j-- {
		b = append(b, symbol[j])
	}

	for j := len(ms.currencyPositivePrefix) - 1; j >= 0; j-- {
		b = append(b, ms.currencyPositivePrefix[j])
	}

	if num < 0 {
		b = append(b, ms.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, ms.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'ms_BN'
// in accounting notation.
func (ms *ms_BN) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := ms.currencies[currency]
	l := len(s) + len(symbol) + 4 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, ms.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, ms.group[0])
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {

		for j := len(symbol) - 1; j >= 0; j-- {
			b = append(b, symbol[j])
		}

		for j := len(ms.currencyNegativePrefix) - 1; j >= 0; j-- {
			b = append(b, ms.currencyNegativePrefix[j])
		}

		b = append(b, ms.minus[0])

	} else {

		for j := len(symbol) - 1; j >= 0; j-- {
			b = append(b, symbol[j])
		}

		for j := len(ms.currencyPositivePrefix) - 1; j >= 0; j-- {
			b = append(b, ms.currencyPositivePrefix[j])
		}

	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, ms.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	return string(b)
}

// FmtDateShort returns the short date representation of 't' for 'ms_BN'
func (ms *ms_BN) FmtDateShort(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2f}...)

	if t.Month() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Month()), 10)

	b = append(b, []byte{0x2f}...)

	if t.Year() > 9 {
		b = append(b, strconv.Itoa(t.Year())[2:]...)
	} else {
		b = append(b, strconv.Itoa(t.Year())[1:]...)
	}

	return string(b)
}

// FmtDateMedium returns the medium date representation of 't' for 'ms_BN'
func (ms *ms_BN) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, ms.monthsAbbreviated[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateLong returns the long date representation of 't' for 'ms_BN'
func (ms *ms_BN) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, ms.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateFull returns the full date representation of 't' for 'ms_BN'
func (ms *ms_BN) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Day() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, ms.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'ms_BN'
func (ms *ms_BN) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, ms.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, ms.periodsAbbreviated[0]...)
	} else {
		b = append(b, ms.periodsAbbreviated[1]...)
	}

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'ms_BN'
func (ms *ms_BN) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, ms.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, ms.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, ms.periodsAbbreviated[0]...)
	} else {
		b = append(b, ms.periodsAbbreviated[1]...)
	}

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'ms_BN'
func (ms *ms_BN) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, ms.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, ms.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, ms.periodsAbbreviated[0]...)
	} else {
		b = append(b, ms.periodsAbbreviated[1]...)
	}

	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()
	b = append(b, tz...)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'ms_BN'
func (ms *ms_BN) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, ms.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, ms.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, ms.periodsAbbreviated[0]...)
	} else {
		b = append(b, ms.periodsAbbreviated[1]...)
	}

	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()

	if btz, ok := ms.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	return string(b)
}
