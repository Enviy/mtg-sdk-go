package mtg

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const (
	queryURL = "https://api.magicthegathering.io/v1/"
)

var linkRE = regexp.MustCompile(`<(.*)>; rel="(.*)"`)

type cardColumn string

var (
	// CardName is the column for the name property.
	// For split, double-faced and flip cards, just the name of one side of the card.
	// Basically each ‘sub-card’ has its own record.
	CardName = cardColumn("name")
	// CardLayout is the column for the layout property.
	// The card layout. Possible values: normal, split, flip, double-faced, token,\
	// plane, scheme, phenomenon, leveler, vanguard
	CardLayout = cardColumn("layout")
	// CardCMC is the column for the cmc property.
	// Converted mana cost. Always a number.
	CardCMC = cardColumn("cmc")
	// CardColors is the column for the colors property.
	// The card colors. Usually this is derived from the casting cost, but some\
	// cards are special (like the back of dual sided cards and Ghostfire).
	CardColors = cardColumn("colors")
	// CardColorIdentity is the column for the color identity property.
	// The card colors by color code. [“Red”, “Blue”] becomes [“R”, “U”]
	CardColorIdentity = cardColumn("colorIdentity")
	// CardType is the column for the type property.
	// The card type. This is the type you would see on the card if printed today.
	// NOTE: The dash is a UTF8 'long dash’ as per the MTG rules
	CardType = cardColumn("type")
	// CardSupertypes is the column for the supertypes property.
	// The supertypes of the card. These appear to the far left of the card type.
	// Example:Basic, Legendary, Snow, World, Ongoing
	CardSupertypes = cardColumn("supertypes")
	// CardTypes is the column for the types property.
	// The types of the card. These appear to the left of the dash in a card type.
	// Example:Instant, Sorcery, Artifact, Creature, Enchantment, Land, Planeswalker
	CardTypes = cardColumn("types")
	// CardSubtypes is the column for the subtypes property.
	// The subtypes of the card. Appear to the right of the dash in a card type.
	// Usually each word is its own subtype.
	// Example:Trap, Arcane, Equipment, Aura, Human, Rat, Squirrel, etc.
	CardSubtypes = cardColumn("subtypes")
	// CardRarity is the column for the rarity property.
	// The rarity of the card.
	// Example:Common, Uncommon, Rare, Mythic Rare, Special, Basic Land
	CardRarity = cardColumn("rarity")
	// CardSet is the column for the set property.
	// The set the card belongs to (set code).
	CardSet = cardColumn("set")
	// CardSetName is the column for the setName property.
	// The set the card belongs to.
	CardSetName = cardColumn("setName")
	// CardText is the column for the text property.
	// The oracle text of the card. May contain mana symbols and other symbols.
	CardText = cardColumn("text")
	// CardFlavor is the column for the flavor property.
	// The flavor text of the card.
	CardFlavor = cardColumn("flavor")
	// CardArtist is the column for the artist property.
	// The artist of the card. This may not match what is on the card as MTGJSON\
	// corrects many card misprints.
	CardArtist = cardColumn("artist")
	// CardNumber is the column for the number property.
	// The card number. This is printed at the bottom-center of the card in small\
	// text. This is a string because some cards have letters in their numbers.
	CardNumber = cardColumn("number")
	// CardPower is the column for the power property.
	// The power of the card. This is only present for creatures. This is a string\
	// because some cards have powers like: “1+*”.
	CardPower = cardColumn("power")
	// CardToughness is the column for the toughness property.
	// The toughness of the card. This is only present for creatures. This is a\
	// string, because some cards have toughness like: “1+*”.
	CardToughness = cardColumn("toughness")
	// CardLoyalty is the column for the loyalty property.
	// The loyalty of the card. This is only present for planeswalkers.
	CardLoyalty = cardColumn("loyalty")
	// CardForeignName is the column for the foreign name property.
	// The name of a card in a foreign language it was printed in.
	CardForeignName = cardColumn("foreignName")
	// CardLanguage is the column for the language property.
	// The language the card is printed in. Use this parameter when searching by\
	// foreignName.
	CardLanguage = cardColumn("language")
	// CardGameFormat is the column for the game format property.
	// The game format, such as Commander, Standard, Legacy, etc. (when used,\
	// legality defaults to Legal unless supplied).
	CardGameFormat = cardColumn("gameFormat")
	// CardLegality is the column for the legality property.
	// The legality of the card for a given format; Legal, Banned or Restricted.
	CardLegality = cardColumn("legality")
)

// Query interface can be used to query multiple cards by their properties.
type Query interface {
	// Where filters the given column by the given value
	Where(column cardColumn, query string) Query
	// Sorts the query results by the given column
	OrderBy(column cardColumn) Query
	// Creates a copy of this query
	Copy() Query
	// Fetches all cards matching the current query
	All() ([]*Card, error)
	// Fetches the given page of cards.
	Page(pageNum int) (cards []*Card, totalCardCount int, err error)
	// Fetches one page of cards with a given page size
	PageS(pageNum int, pageSize int) (cards []*Card, totalCardCount int, err error)
	// Fetches some random cards
	Random(count int) ([]*Card, error)
}

// NewQuery creates a new Query to fetch cards.
func NewQuery() Query {
	return make(query)
}

type query map[string]string

func fetchCards(url string) ([]*Card, http.Header, error) {
	// resp is http.Response
	resp, err := http.Get(url)
	if err != nil {
		return nil, nil, err
	}

	// resp.Body is io.ReadCloser
	bdy := resp.Body
	defer bdy.Close()
	if err := checkError(resp); err != nil {
		return nil, nil, err
	}

	cards, err := decodeCards(bdy)
	if err != nil {
		return nil, nil, err
	}

	return cards, resp.Header, nil
}

// decodeCards unmarshals resp body to cardResponse struct.
func decodeCards(reader io.Reader) ([]*Card, error) {
	asBytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	var cardResp cardResponse
	if err := json.Unmarshal(asBytes, &cardResp); err != nil {
		return nil, err
	}
	/*
		decoder := json.NewDecoder(reader)
		err = decoder.Decode(&cardResp)
		if err != nil {
			return nil, err
		}
	*/
	if cardResp.Card != nil {
		return []*Card{cardResp.Card}, nil
	}

	return cardResp.Cards, nil
}

func (q query) All() ([]*Card, error) {
	var allCards []*Card

	queryVals := make(url.Values)
	for k, v := range q {
		queryVals.Set(k, v)
	}
	nextURL := queryURL + "cards?" + queryVals.Encode()
	for nextURL != "" {
		cards, header, err := fetchCards(nextURL)
		if err != nil {
			return nil, err
		}

		// TODO: Investigate this line's impact on workflow.
		nextURL = ""
		if linkH, ok := header["Link"]; ok {
			parts := strings.Split(linkH[0], ",")
			for _, link := range parts {
				match := linkRE.FindStringSubmatch(link)
				if match != nil {
					if match[2] == "next" {
						nextURL = match[1]
					}
				}
			}
		}

		allCards = append(allCards, cards...)
	}
	return allCards, nil
}

func (q query) Page(pageNum int) ([]*Card, int, error) {
	return q.PageS(pageNum, 100)
}

func (q query) PageS(pageNum int, pageSize int) ([]*Card, int, error) {
	var cards []*Card
	totalCardCount := 0

	queryVals := make(url.Values)
	for k, v := range q {
		queryVals.Set(k, v)
	}

	queryVals.Set("page", strconv.Itoa(pageNum))
	queryVals.Set("pageSize", strconv.Itoa(pageSize))

	url := queryURL + "cards?" + queryVals.Encode()
	cards, header, err := fetchCards(url)
	if err != nil {
		return nil, 0, err
	}

	totalCardCount = len(cards)
	if totals, ok := header["Total-Count"]; ok && len(totals) > 0 {
		if totalCardCount, err = strconv.Atoi(totals[0]); err != nil {
			return nil, 0, err
		}
	}

	return cards, totalCardCount, nil
}

// Random cards by page size.
func (q query) Random(count int) ([]*Card, error) {
	queryVals := make(url.Values)
	for k, v := range q {
		queryVals.Set(k, v)
	}

	queryVals.Set("random", "true")
	queryVals.Set("pageSize", strconv.Itoa(count))

	url := queryURL + "cards?" + queryVals.Encode()
	cards, _, err := fetchCards(url)
	return cards, err
}

// Copy builds a new map using existing parameters.
func (q query) Copy() Query {
	r := make(query)
	for k, v := range q {
		r[k] = v
	}
	return r
}

// Where adds parameters to a map used in url.Values.
func (q query) Where(column cardColumn, qry string) Query {
	q[string(column)] = qry
	return q
}

func (q query) OrderBy(column cardColumn) Query {
	q["orderBy"] = string(column)
	return q
}
