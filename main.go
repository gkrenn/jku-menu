package main

import (
	"bytes" // Still needed to escape non-description fields
	"fmt"
	"html"
	"log"
	"os"
	"strings"
	"text/template"
)

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
	jkuMensa := fetchJKUMensa()
	khgMenu, err := fetchKHGMenu()
	if err != nil {
		log.Fatalf("Error fetching KHG menu: %v", err)
	}

	// write week html with tabs for all days
	htmlOutput := renderMenusForWeekTabs(*jkuMensa, *khgMenu)
	if err := os.WriteFile("menu_for_week_tabs.html", []byte(htmlOutput), 0644); err != nil {
		log.Fatalf("Error writing week tabs HTML to file: %v", err)
	}
}

func renderMenusForWeekTabs(jkuMensa MenuPlan, khgMenu MenuPlan) string {
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
		"Days": days,
	}
	tmpl, err := template.ParseFiles("menu_for_week_tabs.tmpl")
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
