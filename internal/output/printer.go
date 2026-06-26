// Package output handles formatting and printing scan results.
package output

import (
	"fmt"
	"sort"
	"strings"

	"free-api-hunter/internal/models"
)

// PrintResults prints scan findings and providers to stdout.
func PrintResults(raw []models.Finding, filtered []models.Finding, providers []*models.Provider, limit int) {
	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("FREE API HUNTER — РЕЗУЛЬТАТЫ")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Сырых находок: %d\n", len(raw))
	fmt.Printf("После фильтра: %d\n", len(filtered))
	fmt.Printf("Провайдеров в базе: %d\n", len(providers))
	fmt.Println(strings.Repeat("-", 60))

	// Топ находок
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].QualityScore > filtered[j].QualityScore
	})

	topN := limit
	if topN > len(filtered) {
		topN = len(filtered)
	}
	fmt.Printf("\nТоп %d находок:\n", topN)
	for i := 0; i < topN; i++ {
		f := filtered[i]
		fmt.Printf("%d. [%.2f] %s\n", i+1, f.QualityScore, f.Title)
		fmt.Printf("   Источник: %s\n", f.SourceID)
		fmt.Printf("   URL: %s\n", f.URL)
		desc := f.Description
		if len(desc) > 150 {
			desc = desc[:150] + "..."
		}
		fmt.Printf("   Описание: %s\n", desc)
		fmt.Println()
	}

	// Провайдеры по приоритету (verified + confirmed, без credit card)
	var highPri []*models.Provider
	for _, p := range providers {
		if (p.Status == models.StatusVerified || p.Status == models.StatusConfirmed) && !p.CreditCard {
			highPri = append(highPri, p)
		}
	}
	sort.Slice(highPri, func(i, j int) bool {
		return highPri[i].Name < highPri[j].Name
	})

	fmt.Printf("\nПодтверждённых бесплатных провайдеров: %d\n", len(highPri))
	if len(highPri) == 0 {
		fmt.Println("  Нет подтверждённых провайдеров.")
	}
	for _, p := range highPri {
		modelsStr := "N/A"
		if len(p.Models) > 0 {
			modelsStr = strings.Join(p.Models, ", ")
		}
		limitsStr := p.Limits
		if limitsStr == "" {
			limitsStr = "не указаны"
		}
		fmt.Printf("  • %s\n", p.Name)
		fmt.Printf("    Модели: %s\n", modelsStr)
		fmt.Printf("    Лимиты: %s\n", limitsStr)
		fmt.Printf("    URL: %s\n", p.URL)
		fmt.Println()
	}
}
