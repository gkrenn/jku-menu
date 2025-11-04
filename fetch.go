package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	jkuMensaURL = "https://backend.mensen.at/api"
	khgMenuURL  = "https://www.dioezese-linz.at/khg/mensa/menueplan"
)

func fetchJKUMensa() *MenuPlan {
	apiUrl := jkuMensaURL
	query := `query Location($locationUri: String!, $weekDay: String!) {
	  nodeByUri(uri: $locationUri) {
		... on Location {
		  databaseId
		  featuredImage {
			node {
			  altText
			  caption(format: RENDERED)
			  sourceUrl
			}
		  }
		  locationData {
			address {
			  city
			  country
			  line {
				one
				two
			  }
			  state
			  street {
				name
				number
			  }
			  zipCode
			}
			contact {
			  emailAddress
			  phoneNumber
			}
			contactPerson {
			  emailAddress
			  firstname
			  lastname
			  phoneNumber
			  position
			}
			delegateId
			menuplanAltInformation
			menuplanAvailable
			gallery {
			  nodes {
				altText
				caption(format: RENDERED)
				sourceUrl
			  }
			}
			noticeAccess
			noticeDirection
			noticeLocation
		  }
		  locationServices {
			nodes {
			  locationServiceData {
				iconslug
			  }
			  name
			}
		  }
		  locationStates {
			nodes {
			  name
			  slug
			}
		  }
		  menuplanCurrentWeek
		  menuplanNextWeek
		  openingHour(day: $weekDay) {
			nowDate
			nowWeekDay
			status
			from
			to
			closed
			reopen
		  }
		  openingHours {
			monday
			tuesday
			thursday
			wednesday
			friday
			saturday
			sunday
			notice
			exceptions
		  }
		  title
		  uri
		  seo {
			canonicalUrl
			description
			focusKeywords
			robots
			title
			openGraph {
			  alternateLocales
			  description
			  image {
				height
				secureUrl
				type
				url
				width
			  }
			  title
			  updatedTime
			  url
			  siteName
			  type
			}
		  }
		}
	  }
	}`

	payload := GraphQLRequest{
		Query: query,
		Variables: Variables{
			LocationURI: "standort/mensa-jku/",
			WeekDay:     "now",
		},
		OperationName: "Location",
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Fatalf("Error marshaling request payload: %v", err)
	}

	req, err := http.NewRequest("POST", apiUrl, bytes.NewBuffer(payloadBytes))
	if err != nil {
		log.Fatalf("Error creating HTTP request: %v", err)
	}

	req.Header.Set("accept", "*/*")
	req.Header.Set("accept-language", "en-US,en;q=0.8")
	req.Header.Set("cache-control", "no-cache")
	req.Header.Set("content-type", "application/json")
	req.Header.Set("origin", "https://www.mensen.at")
	req.Header.Set("pragma", "no-cache")
	req.Header.Set("priority", "u=1, i")
	req.Header.Set("referer", "https://www.mensen.at/")
	req.Header.Set("sec-ch-ua", `"Chromium";v="142", "Brave";v="142", "Not_A Brand";v="99"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"macOS"`)
	req.Header.Set("sec-fetch-dest", "empty")
	req.Header.Set("sec-fetch-mode", "cors")
	req.Header.Set("sec-fetch-site", "same-site")
	req.Header.Set("sec-gpc", "1")
	req.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Error sending HTTP request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Error reading response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("API request failed with status: %s\nResponse: %s", resp.Status, string(body))
	}

	var apiResponse APIResponse
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		log.Fatalf("Error unmarshaling outer JSON: %v\nBody: %s", err, string(body))
	}

	var currentWeekMenu MenuPlan
	menuString := apiResponse.Data.NodeByUri.MenuplanCurrentWeek
	if err := json.Unmarshal([]byte(menuString), &currentWeekMenu); err != nil {
		log.Fatalf("Error unmarshaling inner menu JSON: %v\nString was: %s", err, menuString)
	}

	return &currentWeekMenu
}

// getDayKey converts the German day name to a numeric string key.
func getDayKey(day string) string {
	switch strings.ToLower(day) {
	case "montag":
		return "1"
	case "dienstag":
		return "2"
	case "mittwoch":
		return "3"
	case "donnerstag":
		return "4"
	case "freitag":
		return "5"
	case "samstag":
		return "6"
	case "sonntag":
		return "7"
	default:
		return "" // Invalid day
	}
}

var (
	reWeek = regexp.MustCompile(`KW (\d+)`)
	reYear = regexp.MustCompile(`(\d{4})`)
)

func fetchKHGMenu() (*MenuPlan, error) {
	url := khgMenuURL
	res, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL %s: %w", url, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status code: %d", res.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	menuPlan := MenuPlan{
		Menus: []MenuCategory{
			{Name: "Menü 1", Menus: make(map[string][]Dish)},
			{Name: "Menü 2", Menus: make(map[string][]Dish)},
		},
	}

	headerText := doc.Find(".swslang h4").First().Text()

	if weekMatches := reWeek.FindStringSubmatch(headerText); len(weekMatches) > 1 {
		menuPlan.Week = weekMatches[1]
	}
	if yearMatches := reYear.FindStringSubmatch(headerText); len(yearMatches) > 1 {
		if year, err := strconv.Atoi(yearMatches[1]); err == nil {
			menuPlan.Year = year
		}
	}

	// Process the menu table
	var currentDayKey string
	var dishCounterForDay int // 0 for Menü 1, 1 for Menü 2

	doc.Find("table.sweTable1 tbody tr").Each(func(i int, row *goquery.Selection) {

		// Day header row (e.g., "Montag")
		if row.HasClass("sweTableRow1") {
			dayName := row.Find("strong").Text()
			currentDayKey = getDayKey(dayName)
			dishCounterForDay = 0
			return
		}

		// Dish row: has 3 <td> children
		cells := row.Find("td")
		if cells.Length() == 3 && currentDayKey != "" {
			title := strings.TrimSpace(cells.Eq(0).Text())
			price := strings.TrimSpace(cells.Eq(1).Text())
			dish := Dish{
				TitleDe: title,
				Price:   price,
			}
			if dishCounterForDay < len(menuPlan.Menus) {
				category := &menuPlan.Menus[dishCounterForDay]
				category.Menus[currentDayKey] = append(category.Menus[currentDayKey], dish)
				dishCounterForDay++
			}
		}
	})

	return &menuPlan, nil
}
