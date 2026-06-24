package main

import (
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/jackielii/structpages"
)

// dashboardData holds all data the dashboard page may need.
// It is the props type for (dashboard).Page and is referenced in pages.gsx.
type dashboardData struct {
	Stats         UserStats
	Sales         SalesData
	Notifications []Notification
}

type UserStats struct {
	ActiveUsers int
	NewToday    int
}

type SalesData struct {
	Points []DataPoint
	Total  float64
}

type DataPoint struct {
	Label string
	Value int
}

type Notification struct {
	Message string
	Time    time.Time
}

// Props demonstrates conditional data loading with RenderTarget.
// Returns dashboardData directly — gsx now emits method components as
// func (p dashboard) Page(d dashboardData) gsx.Node without any wrapper struct.
func (p dashboard) Props(r *http.Request, target structpages.RenderTarget) (dashboardData, error) {
	switch {
	case target.Is(UserStatsWidget):
		stats := loadUserStats()
		return dashboardData{}, structpages.RenderComponent(UserStatsWidget(stats))

	case target.Is(SalesChartWidget):
		sales := loadSalesData()
		return dashboardData{}, structpages.RenderComponent(SalesChartWidget(sales))

	case target.Is(NotificationsList):
		notifications := loadNotifications()
		return dashboardData{}, structpages.RenderComponent(NotificationsList(NotificationsListProps{Notifications: notifications}))

	case target.Is(p.Page):
		return dashboardData{
			Stats:         loadUserStats(),
			Sales:         loadSalesData(),
			Notifications: loadNotifications(),
		}, nil

	default:
		return dashboardData{
			Stats:         loadUserStats(),
			Sales:         loadSalesData(),
			Notifications: loadNotifications(),
		}, nil
	}
}

// Mock data loaders (simulating database queries).
func loadUserStats() UserStats {
	return UserStats{
		ActiveUsers: 1000 + rand.IntN(500),
		NewToday:    10 + rand.IntN(90),
	}
}

func loadSalesData() SalesData {
	points := []DataPoint{
		{Label: "Mon", Value: 30 + rand.IntN(100)},
		{Label: "Tue", Value: 30 + rand.IntN(100)},
		{Label: "Wed", Value: 30 + rand.IntN(100)},
		{Label: "Thu", Value: 30 + rand.IntN(100)},
		{Label: "Fri", Value: 30 + rand.IntN(100)},
	}
	total := 0.0
	for _, pt := range points {
		total += float64(pt.Value) * 100.0
	}
	return SalesData{
		Points: points,
		Total:  total,
	}
}

func loadNotifications() []Notification {
	messages := []string{
		"New user registered",
		"Payment received",
		"System update available",
		"New order placed",
		"Report generated",
		"Backup completed",
	}
	count := 3 + rand.IntN(3)
	notifications := make([]Notification, count)
	for i := 0; i < count; i++ {
		notifications[i] = Notification{
			Message: messages[rand.IntN(len(messages))],
			Time:    time.Now().Add(-time.Duration(rand.IntN(120)) * time.Minute),
		}
	}
	return notifications
}
