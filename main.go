package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"text/tabwriter"
	"os"
	"sort"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/cloudbilling/v1"
	"google.golang.org/api/option"
)

type SpotPricing struct {
	Name     string
	PriceUSD float64
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func run() error {
	ctx := context.Background()

	client, err := google.DefaultClient(ctx, cloudbilling.CloudPlatformScope)
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}

	billingService, err := cloudbilling.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("failed to create billing service: %v", err)
	}

	services, err := billingService.Services.List().Do()
	if err != nil {
		return fmt.Errorf("failed to list services: %v", err)
	}

	var computeServiceID string
	for _, service := range services.Services {
		if strings.Contains(strings.ToLower(service.DisplayName), "compute engine") {
			computeServiceID = service.Name
			break
		}
	}

	if computeServiceID == "" {
		return fmt.Errorf("compute engine service not found")
	}

	pricingInfo, err := billingService.Services.Skus.List(computeServiceID).Do()
	if err != nil {
		return fmt.Errorf("failed to list SKUs: %v", err)
	}

	var spotPrices []SpotPricing

	for _, sku := range pricingInfo.Skus {
		if sku.Category.ResourceFamily == "Compute" &&
			strings.Contains(strings.ToLower(sku.Category.UsageType), "preemptible") &&
			(strings.Contains(sku.Description, "Tokyo") || strings.Contains(sku.Description, "Japan")) {
			for _, pricingInfo := range sku.PricingInfo {
				for _, pricingExpression := range pricingInfo.PricingExpression.TieredRates {
					spotPrice := SpotPricing{
						Name:     sku.Description,
						PriceUSD: float64(pricingExpression.UnitPrice.Units) + float64(pricingExpression.UnitPrice.Nanos)/1e9,
					}
					spotPrices = append(spotPrices, spotPrice)
					break // We're only interested in the first tier
				}
			}
		}
	}

	// Sort spotPrices by PriceUSD
	sort.Slice(spotPrices, func(i, j int) bool {
		return spotPrices[i].PriceUSD < spotPrices[j].PriceUSD
	})

	// Print with equal indent using tabwriter
	fmt.Println("Spot VM Prices in Tokyo (ordered by price):")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, price := range spotPrices {
		fmt.Fprintf(w, "%s\t$%.6f per hour\n", price.Name, price.PriceUSD)
	}
	w.Flush()

	return nil
}
