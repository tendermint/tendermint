package et

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type et struct {
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
	currencyPositiveSuffix string
	currencyNegativePrefix string
	currencyNegativeSuffix string
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

// New returns a new instance of translator for the 'et' locale
func New() locales.Translator {
	return &et{
		locale:                 "et",
		pluralsCardinal:        []locales.PluralRule{2, 6},
		pluralsOrdinal:         []locales.PluralRule{6},
		pluralsRange:           []locales.PluralRule{6},
		decimal:                ",",
		group:                  " ",
		minus:                  "−",
		percent:                "%",
		perMille:               "‰",
		timeSeparator:          ":",
		inifinity:              "∞",
		currencies:             []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "AU$", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "R$", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CA$", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CN¥", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "kr", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "€", "FIM", "FJD", "FKP", "FRF", "£", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HK$", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "₪", "₹", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "¥", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "₩", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MX$", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZ$", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "฿", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "NT$", "TZS", "UAH", "UAK", "UGS", "UGX", "$", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "₫", "VNN", "VUV", "WST", "FCFA", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "EC$", "XDR", "XEU", "XFO", "XFU", "CFA", "XPD", "CFPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		currencyPositiveSuffix: " ",
		currencyNegativePrefix: "(",
		currencyNegativeSuffix: " )",
		monthsAbbreviated:      []string{"", "jaan", "veebr", "märts", "apr", "mai", "juuni", "juuli", "aug", "sept", "okt", "nov", "dets"},
		monthsNarrow:           []string{"", "J", "V", "M", "A", "M", "J", "J", "A", "S", "O", "N", "D"},
		monthsWide:             []string{"", "jaanuar", "veebruar", "märts", "aprill", "mai", "juuni", "juuli", "august", "september", "oktoober", "november", "detsember"},
		daysAbbreviated:        []string{"P", "E", "T", "K", "N", "R", "L"},
		daysNarrow:             []string{"P", "E", "T", "K", "N", "R", "L"},
		daysShort:              []string{"P", "E", "T", "K", "N", "R", "L"},
		daysWide:               []string{"pühapäev", "esmaspäev", "teisipäev", "kolmapäev", "neljapäev", "reede", "laupäev"},
		periodsAbbreviated:     []string{"AM", "PM"},
		periodsNarrow:          []string{"AM", "PM"},
		periodsWide:            []string{"AM", "PM"},
		erasAbbreviated:        []string{"eKr", "pKr"},
		erasNarrow:             []string{"eKr", "pKr"},
		erasWide:               []string{"enne Kristust", "pärast Kristust"},
		timezones:              map[string]string{"ART": "Argentina standardaeg", "CAT": "Kesk-Aafrika aeg", "OEZ": "Ida-Euroopa standardaeg", "EST": "Idaranniku standardaeg", "ECT": "Ecuadori aeg", "HAST": "Hawaii-Aleuudi standardaeg", "COT": "Colombia standardaeg", "COST": "Colombia suveaeg", "NZST": "Uus-Meremaa standardaeg", "HEPMX": "Mehhiko Vaikse ookeani suveaeg", "AWDT": "Lääne-Austraalia suveaeg", "CST": "Kesk-Ameerika standardaeg", "CDT": "Kesk-Ameerika suveaeg", "WITA": "Kesk-Indoneesia aeg", "HEOG": "Lääne-Gröönimaa suveaeg", "ACWST": "Kesk-Lääne Austraalia standardaeg", "LHDT": "Lord Howe’i suveaeg", "HNNOMX": "Loode-Mehhiko standardaeg", "AEST": "Ida-Austraalia standardaeg", "HEEG": "Ida-Gröönimaa suveaeg", "ACDT": "Kesk-Austraalia suveaeg", "WIB": "Lääne-Indoneesia aeg", "GMT": "Greenwichi aeg", "UYST": "Uruguay suveaeg", "HADT": "Hawaii-Aleuudi suveaeg", "NZDT": "Uus-Meremaa suveaeg", "CHAST": "Chathami standardaeg", "HENOMX": "Loode-Mehhiko suveaeg", "HKST": "Hongkongi suveaeg", "ACWDT": "Kesk-Lääne Austraalia suveaeg", "TMT": "Türkmenistani standardaeg", "MEZ": "Kesk-Euroopa standardaeg", "BT": "Bhutani aeg", "HKT": "Hongkongi standardaeg", "HNPM": "Saint-Pierre’i ja Miqueloni standardaeg", "LHST": "Lord Howe’i standardaeg", "HNOG": "Lääne-Gröönimaa standardaeg", "WESZ": "Lääne-Euroopa suveaeg", "CHADT": "Chathami suveaeg", "MST": "MST", "∅∅∅": "Acre suveaeg", "WIT": "Ida-Indoneesia aeg", "BOT": "Boliivia aeg", "HNT": "Newfoundlandi standardaeg", "AKST": "Alaska standardaeg", "HNEG": "Ida-Gröönimaa standardaeg", "HNPMX": "Mehhiko Vaikse ookeani standardaeg", "PST": "Vaikse ookeani standardaeg", "MESZ": "Kesk-Euroopa suveaeg", "WARST": "Lääne-Argentina suveaeg", "ARST": "Argentina suveaeg", "WAST": "Lääne-Aafrika suveaeg", "HNCU": "Kuuba standardaeg", "HECU": "Kuuba suveaeg", "PDT": "Vaikse ookeani suveaeg", "SRT": "Suriname aeg", "OESZ": "Ida-Euroopa suveaeg", "IST": "India aeg", "CLT": "Tšiili standardaeg", "GFT": "Prantsuse Guajaana aeg", "HAT": "Newfoundlandi suveaeg", "GYT": "Guyana aeg", "WEZ": "Lääne-Euroopa standardaeg", "MDT": "MDT", "AWST": "Lääne-Austraalia standardaeg", "JST": "Jaapani standardaeg", "SGT": "Singapuri standardaeg", "ChST": "Tšamorro standardaeg", "HEPM": "Saint-Pierre’i ja Miqueloni suveaeg", "TMST": "Türkmenistani suveaeg", "WAT": "Lääne-Aafrika standardaeg", "CLST": "Tšiili suveaeg", "ACST": "Kesk-Austraalia standardaeg", "JDT": "Jaapani suveaeg", "AST": "Atlandi standardaeg", "EAT": "Ida-Aafrika aeg", "VET": "Venezuela aeg", "AEDT": "Ida-Austraalia suveaeg", "ADT": "Atlandi suveaeg", "SAST": "Lõuna-Aafrika standardaeg", "EDT": "Idaranniku suveaeg", "MYT": "Malaisia \u200b\u200baeg", "UYT": "Uruguay standardaeg", "WART": "Lääne-Argentina standardaeg", "AKDT": "Alaska suveaeg"},
	}
}

// Locale returns the current translators string locale
func (et *et) Locale() string {
	return et.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'et'
func (et *et) PluralsCardinal() []locales.PluralRule {
	return et.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'et'
func (et *et) PluralsOrdinal() []locales.PluralRule {
	return et.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'et'
func (et *et) PluralsRange() []locales.PluralRule {
	return et.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'et'
func (et *et) CardinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)
	i := int64(n)

	if i == 1 && v == 0 {
		return locales.PluralRuleOne
	}

	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'et'
func (et *et) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleOther
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'et'
func (et *et) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {
	return locales.PluralRuleOther
}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (et *et) MonthAbbreviated(month time.Month) string {
	return et.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (et *et) MonthsAbbreviated() []string {
	return et.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (et *et) MonthNarrow(month time.Month) string {
	return et.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (et *et) MonthsNarrow() []string {
	return et.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (et *et) MonthWide(month time.Month) string {
	return et.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (et *et) MonthsWide() []string {
	return et.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (et *et) WeekdayAbbreviated(weekday time.Weekday) string {
	return et.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (et *et) WeekdaysAbbreviated() []string {
	return et.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (et *et) WeekdayNarrow(weekday time.Weekday) string {
	return et.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (et *et) WeekdaysNarrow() []string {
	return et.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (et *et) WeekdayShort(weekday time.Weekday) string {
	return et.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (et *et) WeekdaysShort() []string {
	return et.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (et *et) WeekdayWide(weekday time.Weekday) string {
	return et.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (et *et) WeekdaysWide() []string {
	return et.daysWide
}

// Decimal returns the decimal point of number
func (et *et) Decimal() string {
	return et.decimal
}

// Group returns the group of number
func (et *et) Group() string {
	return et.group
}

// Group returns the minus sign of number
func (et *et) Minus() string {
	return et.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'et' and handles both Whole and Real numbers based on 'v'
func (et *et) FmtNumber(num float64, v uint64) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 4 + 2*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, et.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				for j := len(et.group) - 1; j >= 0; j-- {
					b = append(b, et.group[j])
				}
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		for j := len(et.minus) - 1; j >= 0; j-- {
			b = append(b, et.minus[j])
		}
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	return string(b)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'et' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (et *et) FmtPercent(num float64, v uint64) string {
	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 5
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, et.decimal[0])
			continue
		}

		b = append(b, s[i])
	}

	if num < 0 {
		for j := len(et.minus) - 1; j >= 0; j-- {
			b = append(b, et.minus[j])
		}
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	b = append(b, et.percent...)

	return string(b)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'et'
func (et *et) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := et.currencies[currency]
	l := len(s) + len(symbol) + 6 + 2*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, et.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				for j := len(et.group) - 1; j >= 0; j-- {
					b = append(b, et.group[j])
				}
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		for j := len(et.minus) - 1; j >= 0; j-- {
			b = append(b, et.minus[j])
		}
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, et.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	b = append(b, et.currencyPositiveSuffix...)

	b = append(b, symbol...)

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'et'
// in accounting notation.
func (et *et) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := et.currencies[currency]
	l := len(s) + len(symbol) + 8 + 2*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, et.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				for j := len(et.group) - 1; j >= 0; j-- {
					b = append(b, et.group[j])
				}
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {

		b = append(b, et.currencyNegativePrefix[0])

	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, et.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	if num < 0 {
		b = append(b, et.currencyNegativeSuffix...)
		b = append(b, symbol...)
	} else {

		b = append(b, et.currencyPositiveSuffix...)
		b = append(b, symbol...)
	}

	return string(b)
}

// FmtDateShort returns the short date representation of 't' for 'et'
func (et *et) FmtDateShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Day() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2e}...)

	if t.Month() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Month()), 10)

	b = append(b, []byte{0x2e}...)

	if t.Year() > 9 {
		b = append(b, strconv.Itoa(t.Year())[2:]...)
	} else {
		b = append(b, strconv.Itoa(t.Year())[1:]...)
	}

	return string(b)
}

// FmtDateMedium returns the medium date representation of 't' for 'et'
func (et *et) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2e, 0x20}...)
	b = append(b, et.monthsAbbreviated[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateLong returns the long date representation of 't' for 'et'
func (et *et) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2e, 0x20}...)
	b = append(b, et.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateFull returns the full date representation of 't' for 'et'
func (et *et) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, et.daysWide[t.Weekday()]...)
	b = append(b, []byte{0x2c, 0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2e, 0x20}...)
	b = append(b, et.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'et'
func (et *et) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, et.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'et'
func (et *et) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, et.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, et.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'et'
func (et *et) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, et.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, et.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()
	b = append(b, tz...)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'et'
func (et *et) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, et.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, et.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()

	if btz, ok := et.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	return string(b)
}
