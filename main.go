package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"

	"github.com/microsoft/azure-devops-go-api/azuredevops"
	"github.com/microsoft/azure-devops-go-api/azuredevops/work"

	_ "github.com/mattn/go-sqlite3"
)

type CapacityData struct {
	Teams                        []TeamData `json:"teams"`
	TotalIterationCapacityPerDay float64    `json:"totalIterationCapacityPerDay"`
	TotalIterationDaysOff        int        `json:"totalIterationDaysOff"`
}

type TeamData struct {
	TeamId             string  `json:"teamId"`
	TeamCapacityPerDay float64 `json:"teamCapacityPerDay"`
	TeamTotalDaysOff   int     `json:"teamTotalDaysOff"`
}

func extractSprintNumber(iterationName *string) (int, error) {
	if iterationName == nil {
		return 0, fmt.Errorf("iteration name is nil")
	}
	re := regexp.MustCompile(`Sprint\s+(\d+)`)
	matches := re.FindStringSubmatch(*iterationName)
	if len(matches) < 2 {
		return 0, fmt.Errorf("iteration name does not contain sprint number")
	}

	sprintNum, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("could not parse sprint number from iteration name: %v", err)
	}

	return sprintNum, nil
}

func createAuthHeader(patToken string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(":" + patToken))
	return "Basic " + encoded
}

func fetchIterationCapacity(connection *azuredevops.Connection, patToken, project, iterationID string) (CapacityData, error) {
	ctx := context.Background()
	client := &http.Client{}

	// Build URL for the capacity API
	capacitiesAPIURL := fmt.Sprintf("%s/%s/_apis/work/iterations/%s/iterationcapacities?api-version=7.0", connection.BaseUrl, project, iterationID)

	// Create a new HTTP request with the correct headers
	req, err := http.NewRequestWithContext(ctx, "GET", capacitiesAPIURL, nil)
	if err != nil {
		return CapacityData{}, err
	}

	// Add authorization header
	authHeader := createAuthHeader(patToken)
	req.Header.Set("Authorization", authHeader)

	// Send the HTTP request and read the response
	resp, err := client.Do(req)
	if err != nil {
		return CapacityData{}, err
	}
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return CapacityData{}, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Read the response body and unmarshal it into a CapacityData struct
	var capacityData CapacityData
	err = json.NewDecoder(resp.Body).Decode(&capacityData)
	if err != nil {
		return CapacityData{}, err
	}

	return capacityData, nil
}

func fetchIterations(connection *azuredevops.Connection, project, team string) ([]work.TeamSettingsIteration, error) {
	ctx := context.Background()
	workClient, err := work.NewClient(ctx, connection)
	if err != nil {
		return nil, err
	}

	// timeframe := string(work.TimeFrameValues.Current)
	iterations, err := workClient.GetTeamIterations(ctx, work.GetTeamIterationsArgs{
		Project:   &project,
		Team:      &team,
		Timeframe: nil, //&timeframe,
	})
	if err != nil {
		return nil, err
	}
	return *iterations, nil
}

type Args struct {
	OrgURL      string `json:"orgURL"`
	Token       string `json:"token"`
	Project     string `json:"project"`
	Team        string `json:"team"`
	SprintStart int    `json:"sprintStart"`
}

type PointsCompleted struct {
	SprintNumber int  `json:"sprint"`
	Completed    int  `json:"completed"`
	Calculate    bool `json:"calculate"`
}

func readPointsCompletedFile(filename string) ([]PointsCompleted, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var pointsData []PointsCompleted
	err = json.NewDecoder(file).Decode(&pointsData)
	if err != nil {
		return nil, err
	}

	return pointsData, nil
}

func findPointsCompleted(sprintNumber int, pointsData []PointsCompleted) int {
	for _, points := range pointsData {
		if points.Calculate && points.SprintNumber == sprintNumber {
			return points.Completed
		}
	}
	return -1 // Sprint not found
}

func readArgsFile(filename string) (Args, error) {
	file, err := os.Open(filename)
	if err != nil {
		return Args{}, err
	}
	defer file.Close()

	var args Args
	err = json.NewDecoder(file).Decode(&args)
	if err != nil {
		return Args{}, err
	}

	return args, nil
}

func main() {

	pointsData, err := readPointsCompletedFile("points_completed.json")
	if err != nil {
		fmt.Println("Error reading points_completed.json:", err)
		os.Exit(1)
	}

	for _, points := range pointsData {
		fmt.Printf("Sprint %d completed %d points\n", points.SprintNumber, points.Completed)
	}

	args, err := readArgsFile("arguments.json")
	if err != nil {
		fmt.Println("Error reading arguments.json:", err)
		os.Exit(1)
	}

	orgURL := args.OrgURL
	token := args.Token
	project := args.Project
	team := args.Team
	sprintStart := args.SprintStart

	// Open a new database file
	db, err := sql.Open("sqlite3", "./data.db")
	if err != nil {
		fmt.Println("Error opening database:", err)
		return
	}
	defer db.Close()

	// Create a new table to store iteration capacities
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS iteration_capacity (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT,
		sprint_number INTEGER,
		days_available REAL,
		capacity_per_day REAL,
		days_off INTEGER,
		points_completed INTEGER
	)`)
	if err != nil {
		fmt.Println("Error creating table:", err)
		return
	}

	daysInSprint := 14.0

	connection := azuredevops.NewPatConnection(orgURL, token)
	iterations, err := fetchIterations(connection, project, team)
	if err != nil {
		fmt.Println("Error fetching iterations:", err)
		os.Exit(1)
	}

	for _, iteration := range iterations {

		sprintNum, err := extractSprintNumber(iteration.Name)
		if err != nil {
			fmt.Printf("Error extracting sprint number from iteration name '%s': %v\n", *iteration.Name, err)
			continue
		}

		if sprintNum >= sprintStart {

			fmt.Printf("Working on sprint: %d\n", sprintNum)

			// Fetch iteration capacity details
			capacityData, err := fetchIterationCapacity(connection, token, project, iteration.Id.String())
			if err != nil {
				fmt.Printf("Error fetching capacities for iteration '%s': %v\n", *iteration.Name, err)
				continue
			}

			// Insert a new row into the table
			_, err = db.Exec(`INSERT INTO iteration_capacity (
				name, sprint_number, days_available, capacity_per_day, days_off, points_completed
			) VALUES (?, ?, ?, ?, ?, ?)`,
				iteration.Name,
				sprintNum,
				(capacityData.TotalIterationCapacityPerDay*daysInSprint)-float64(capacityData.TotalIterationDaysOff),
				capacityData.TotalIterationCapacityPerDay,
				capacityData.TotalIterationDaysOff,
				findPointsCompleted(sprintNum, pointsData))
			if err != nil {
				fmt.Println("Error inserting row:", err)
				return
			}
		}
	}

	// Select all rows from the table and print them
	rows, err := db.Query("SELECT * FROM iteration_capacity")
	if err != nil {
		fmt.Println("Error selecting rows:", err)
		return
	}
	defer rows.Close()

	var id int
	var name string
	var sprint_number int
	var days_available float64
	var capacity_per_day float64
	var days_off int
	var points_completed int

	for rows.Next() {
		err := rows.Scan(&id, &name, &sprint_number, &days_available, &capacity_per_day, &days_off, &points_completed)
		if err != nil {
			fmt.Println("Error scanning row:", err)
			return
		}
		fmt.Printf("ID: %d\n", id)
		fmt.Printf("Name: %s\n", name)
		fmt.Printf("Sprint: %d\n", sprint_number)
		fmt.Printf("Days Available: %f\n", days_available)
		fmt.Printf("Capacity Per Day: %f\n", capacity_per_day)
		fmt.Printf("Days Off: %d\n", days_off)
		fmt.Printf("Points Completed: %d\n", points_completed)
		fmt.Println()
	}
}
