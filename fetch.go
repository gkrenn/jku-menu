package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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

func fetchJKUMensa() (MenuPlan, error) {
	apiUrl := jkuMensaURL
	query := `query Location($locationUri: String!, $weekDay: String!) {
	  nodeByUri(uri: $locationUri) {
		... on Location {
		  menuplanCurrentWeek
		  openingHour(day: $weekDay) {
			nowDate
			nowWeekDay
			status
			from
			to
			closed
			reopen
		  }
		  title
		  uri
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
		return MenuPlan{}, fmt.Errorf("error marshaling request payload: %w", err)
	}

	req, err := http.NewRequest("POST", apiUrl, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return MenuPlan{}, fmt.Errorf("error creating HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return MenuPlan{}, fmt.Errorf("error sending HTTP request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return MenuPlan{}, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return MenuPlan{}, fmt.Errorf("API request failed with status: %s\nResponse: %s", resp.Status, string(body))
	}

	var apiResponse APIResponse
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return MenuPlan{}, fmt.Errorf("error unmarshaling outer JSON: %w\nBody: %s", err, string(body))
	}

	var currentWeekMenu MenuPlan
	menuString := apiResponse.Data.NodeByUri.MenuplanCurrentWeek
	if err := json.Unmarshal([]byte(menuString), &currentWeekMenu); err != nil {
		return MenuPlan{}, fmt.Errorf("error unmarshaling inner menu JSON: %w\nString was: %s", err, menuString)
	}

	return currentWeekMenu, nil
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

func fetchKHGMenu() (MenuPlan, error) {
	url := khgMenuURL
	res, err := http.Get(url)
	if err != nil {
		return MenuPlan{}, fmt.Errorf("failed to fetch URL %s: %w", url, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return MenuPlan{}, fmt.Errorf("bad status code: %d", res.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return MenuPlan{}, fmt.Errorf("failed to parse HTML: %w", err)
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

	return menuPlan, nil
}
