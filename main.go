package main

import (
	"bytes"
	"flag"
	"fmt"
	"html"
	"log"
	"os"
	"strings"
	"text/template"
	"time"

	_ "embed"
)

//go:embed menu_for_week_tabs.tmpl
var menuForWeekTabsTemplate string

type GraphQLRequest struct {
	Query         string    `json:"query"`
	Variables     Variables `json:"variables"`
	OperationName string    `json:"operationName"`
}

type Variables struct {
	LocationURI string `json:"locationUri"`
	WeekDay     string `json:"weekDay"`
}

// APIResponse matches the outer JSON structure
type APIResponse struct {
	Data struct {
		NodeByUri struct {
			Title               string `json:"title"`
			MenuplanCurrentWeek string `json:"menuplanCurrentWeek"` // This is stringified JSON
			MenuplanNextWeek    string `json:"menuplanNextWeek"`    // Stringified JSON, may be empty
		} `json:"nodeByUri"`
	} `json:"data"`
}

// MenuPlan matches the inner, stringified JSON structure
type MenuPlan struct {
	Week  string         `json:"week"`
	Year  int            `json:"year"`
	Menus []MenuCategory `json:"menus"`
}

type MenuCategory struct {
	Name  string            `json:"name"`
	Menus map[string][]Dish `json:"menus"` // Key is the day of the week ("1", "2", etc.)
}

type Dish struct {
	TitleDe string `json:"title_de"`
	Price   string `json:"price"`
}

func main() {
	outputFile := flag.String("o", "index.html", "Output filename (default: index.html)")
	dumpRaw := flag.Bool("dump", false, "Dump raw JKU API response to jku_raw_response.json and exit")
	flag.Parse()

	if *dumpRaw {
		raw, err := FetchJKURaw()
		if err != nil {
			log.Fatalf("Error fetching raw JKU response: %v", err)
		}
		if err := os.WriteFile("jku_raw_response.json", raw, 0644); err != nil {
			log.Fatalf("Error writing raw response: %v", err)
		}
		fmt.Println("Raw JKU API response written to jku_raw_response.json")
		return
	}

	jkuCurrentWeek, jkuNextWeek, err := fetchJKUMensa()
	jkuFetchedAt := time.Now()
	if err != nil {
		log.Printf("Error fetching JKU menu: %v", err)
	}

	khgMenu, err := fetchKHGMenu()
	khgFetchedAt := time.Now()
	if err != nil {
		log.Printf("Error fetching KHG menu: %v", err)
	}

	jkuMensa := jkuCurrentWeek
	if time.Now().Weekday() == time.Saturday && len(jkuNextWeek.Menus) > 0 {
		jkuMensa = jkuNextWeek
	}

	htmlOutput := renderMenusForWeekTabs(jkuMensa, khgMenu, jkuFetchedAt, khgFetchedAt)
	if err := os.WriteFile(*outputFile, []byte(htmlOutput), 0644); err != nil {
		log.Fatalf("Error writing week tabs HTML to file: %v", err)
	}
}

func renderMenusForWeekTabs(jkuMensa MenuPlan, khgMenu MenuPlan, jkuFetchedAt time.Time, khgFetchedAt time.Time) string {
	type DishView struct {
		Title string
		Price string
	}
	type CategoryView struct {
		Name   string
		Dishes []DishView
	}
	type MenuView struct {
		Categories []CategoryView
	}
	type DayMenus struct {
		Name     string
		JKUMensa MenuView
		KHG      MenuView
	}
	dayNames := []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday"}
	var days []DayMenus
	for i, dayName := range dayNames {
		dayKey := fmt.Sprintf("%d", i+1)
		getMenuView := func(menu MenuPlan) MenuView {
			var categories []CategoryView
			for _, category := range menu.Menus {
				dishes, dayExists := category.Menus[dayKey]
				if dayExists && len(dishes) > 0 {
					var dishViews []DishView
					for _, dish := range dishes {
						dishViews = append(dishViews, DishView{
							Title: formatTitleForHTML(dish.TitleDe),
							Price: html.EscapeString(dish.Price),
						})
					}
					categories = append(categories, CategoryView{
						Name:   html.EscapeString(category.Name),
						Dishes: dishViews,
					})
				}
			}
			return MenuView{Categories: categories}
		}
		days = append(days, DayMenus{
			Name:     dayName,
			JKUMensa: getMenuView(jkuMensa),
			KHG:      getMenuView(khgMenu),
		})
	}
	data := map[string]interface{}{
		"Days":         days,
		"JKUFetchedAt": jkuFetchedAt.Format("Mon, 02 Jan 2006, 15:04"),
		"KHGFetchedAt": khgFetchedAt.Format("Mon, 02 Jan 2006, 15:04"),
		"JKUWeekInfo":  formatWeekInfo(jkuMensa),
		"KHGWeekInfo":  formatWeekInfo(khgMenu),
	}
	tmpl, err := template.New("menu_for_week_tabs").Parse(menuForWeekTabsTemplate)
	if err != nil {
		return "<h2>Template error.</h2>"
	}
	var buf bytes.Buffer
	tmpl.Execute(&buf, data)
	return buf.String()
}

func formatTitleForHTML(title string) string {
	r := strings.NewReplacer("\n", " ")
	cleaned := r.Replace(title)
	return strings.TrimSpace(cleaned)
}

func formatWeekInfo(menu MenuPlan) string {
	if menu.Week == "" {
		return ""
	}
	if menu.Year > 0 {
		return fmt.Sprintf("KW %s / %d", menu.Week, menu.Year)
	}
	return fmt.Sprintf("KW %s", menu.Week)
}
