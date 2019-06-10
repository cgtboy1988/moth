package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type PuzzleMap struct {
	Points int
	Path   string
}

func (pm *PuzzleMap) MarshalJSON() ([]byte, error) {
	if pm == nil {
		return []byte("null"), nil
	}

	jPath, err := json.Marshal(pm.Path)
	if err != nil {
		return nil, err
	}

	ret := fmt.Sprintf("[%d,%s]", pm.Points, string(jPath))
	return []byte(ret), nil
}

func (ctx *Instance) generatePuzzleList() {
	maxByCategory := map[string]int{}
	completeByCategory := map[string]map[int]int{}
	for _, a := range ctx.PointsLog() {
		if a.Points > maxByCategory[a.Category] {
			maxByCategory[a.Category] = a.Points
		}
		_, ok := completeByCategory[a.Category]
		if !ok {
			completeByCategory[a.Category] = map[int]int{}
		}
		completeByCategory[a.Category][a.Points]++
	}

	ret := map[string][]PuzzleMap{}

	for catName, mb := range ctx.categories {
		
		mf, err := mb.Open("map.txt")
		if err != nil {
			// File isn't in there
			continue
		}
		defer mf.Close()
		
		pointsToDir := map[int]string{}
		
		scanner := bufio.NewScanner(mf)
		for scanner.Scan() {
			line := scanner.Text()

			var pointval int
			var dir string

			n, err := fmt.Sscanf(line, "%d %s", &pointval, &dir)
			if err != nil {
				log.Printf("Parsing map for %s: %v", catName, err)
				continue
			} else if n != 2 {
				log.Printf("Parsing map for %s: short read", catName)
				continue
			}
			
			pointsToDir[pointval] = dir
		}
		
		mf, err = mb.Open("unlockers.txt")
		if err != nil {
			// File isn't in there
			continue
		}
		defer mf.Close()
		
		alreadyUnlocked := map[int]bool{}
		pm := make([]PuzzleMap, 0, 30)
		completed := true
		scanner = bufio.NewScanner(mf)
		for scanner.Scan() {
			line := scanner.Text()

			var pointval int
			//var dir string
			var unlockedBy int

			n, err := fmt.Sscanf(line, "%d %d", &pointval, &unlockedBy)
			if err != nil {
				log.Printf("Parsing map for %s: %v", catName, err)
				continue
			} else if n != 2 {
				log.Printf("Parsing map for %s: short read", catName)
				continue
			}
			
			_, isUnlocked := alreadyUnlocked[pointval]
			if isUnlocked {
				continue
			}
			_, puzzleDone := completeByCategory[catName][pointval]
			
			if puzzleDone {
				pm = append(pm, PuzzleMap{pointval, pointsToDir[pointval]})
				alreadyUnlocked[pointval] = true
			} else {
				completed = false
				_, unlockerDone := completeByCategory[catName][unlockedBy]
				if(unlockedBy == 0 || unlockerDone) {
					pm = append(pm, PuzzleMap{pointval, pointsToDir[pointval]})
					alreadyUnlocked[pointval] = true
				}
			}
		}
		if completed {
			pm = append(pm, PuzzleMap{0, ""})
		}

		ret[catName] = pm
	}

	jpl, err := json.Marshal(ret)
	if err != nil {
		log.Printf("Marshalling puzzles.js: %v", err)
		return
	}
	ctx.jPuzzleList = jpl
	if ctx.options["progression"] == "team" {
		ctx.generatePuzzleListTeams()
	}
}

func (ctx *Instance) generatePuzzleListTeams() {
	
	maxByCategory := map[string]map[string]int{}
	completeByCategory := map[string]map[string]map[int]int{}
	for _, a := range ctx.PointsLog() {
		_, ok := maxByCategory[a.TeamId][a.Category]
		if !ok {
			maxByCategory[a.TeamId]=map[string]int{}
		}
		if a.Points > maxByCategory[a.TeamId][a.Category] {
			maxByCategory[a.TeamId][a.Category] = a.Points
		}
		_, ok = completeByCategory[a.TeamId][a.Category]
		if !ok {
			completeByCategory[a.TeamId] = map[string]map[int]int{}
			completeByCategory[a.TeamId][a.Category] = map[int]int{}
		}
		completeByCategory[a.TeamId][a.Category][a.Points]++
	}

	ret := map[string]map[string][]PuzzleMap{}
	ctx.unlockedPuzzles = map[string]map[string]map[int]bool{}

	for teamName, _ := range ctx.nextAttempt {
		ctx.unlockedPuzzles[teamName] = map[string]map[int]bool{}
		for catName, mb := range ctx.categories {
			ctx.unlockedPuzzles[teamName][catName] = map[int]bool{}
			mf, err := mb.Open("map.txt")
			if err != nil {
				// File isn't in there
				continue
			}
			defer mf.Close()
		
			pointsToDir := map[int]string{}
		
			scanner := bufio.NewScanner(mf)
			for scanner.Scan() {
				line := scanner.Text()

				var pointval int
				var dir string

				n, err := fmt.Sscanf(line, "%d %s", &pointval, &dir)
				if err != nil {
					log.Printf("Parsing map for %s: %v", catName, err)
					continue
				} else if n != 2 {
					log.Printf("Parsing map for %s: short read", catName)
					continue
				}
			
				pointsToDir[pointval] = dir
			}
		
			mf, err = mb.Open("unlockers.txt")
			if err != nil {
				// File isn't in there
				continue
			}
			defer mf.Close()
		
			alreadyUnlocked := map[int]bool{}
			pm := make([]PuzzleMap, 0, 30)
			completed := true
			scanner = bufio.NewScanner(mf)
			for scanner.Scan() {
				line := scanner.Text()

				var pointval int
				//var dir string
				var unlockedBy int

				n, err := fmt.Sscanf(line, "%d %d", &pointval, &unlockedBy)
				if err != nil {
					log.Printf("Parsing map for %s: %v", catName, err)
					continue
				} else if n != 2 {
					log.Printf("Parsing map for %s: short read", catName)
					continue
				}
			
				_, isUnlocked := alreadyUnlocked[pointval]
				if isUnlocked {
					continue
				}
				_, puzzleDone := completeByCategory[teamName][catName][pointval]
			
				if puzzleDone {
					pm = append(pm, PuzzleMap{pointval, pointsToDir[pointval]})
					alreadyUnlocked[pointval] = true
				} else {
					completed = false
					_, unlockerDone := completeByCategory[teamName][catName][unlockedBy]
					if(unlockedBy == 0 || unlockerDone) {
						pm = append(pm, PuzzleMap{pointval, pointsToDir[pointval]})
						alreadyUnlocked[pointval] = true
					}
				}
			}
			if completed {
				pm = append(pm, PuzzleMap{0, ""})
			}
			
			_, ok := ret[teamName]
			if !ok {
				ret[teamName] = map[string][]PuzzleMap{}
			}
			_, ok = ret[teamName][catName]
			if !ok {
				ret[teamName][catName] = []PuzzleMap{}
			}
			ret[teamName][catName] = pm
			ctx.unlockedPuzzles[teamName][catName] = alreadyUnlocked
		}
	}
	
	ctx.jPuzzleListTeam = map[string][]byte{}
	for teamName, _ := range ctx.nextAttempt {
		jpl, err := json.Marshal(ret[teamName])
		if err != nil {
			log.Printf("Marshalling puzzles.js: %v", err)
			return
		}
		ctx.jPuzzleListTeam[teamName] = jpl
	}
}

func (ctx *Instance) generatePointsLog() {
	var ret struct {
		Teams  map[string]string `json:"teams"`
		Points []*Award          `json:"points"`
	}
	ret.Teams = map[string]string{}
	ret.Points = ctx.PointsLog()

	teamNumbersById := map[string]int{}
	for nr, a := range ret.Points {
		teamNumber, ok := teamNumbersById[a.TeamId]
		if !ok {
			teamName, err := ctx.TeamName(a.TeamId)
			if err != nil {
				teamName = "Rodney" // https://en.wikipedia.org/wiki/Rogue_(video_game)#Gameplay
			}
			teamNumber = nr
			teamNumbersById[a.TeamId] = teamNumber
			ret.Teams[strconv.FormatInt(int64(teamNumber), 16)] = teamName
		}
		a.TeamId = strconv.FormatInt(int64(teamNumber), 16)
	}

	jpl, err := json.Marshal(ret)
	if err != nil {
		log.Printf("Marshalling points.js: %v", err)
		return
	}
	ctx.jPointsLog = jpl
}

// maintenance runs
func (ctx *Instance) tidy() {
	// Do they want to reset everything?
	ctx.MaybeInitialize()

	// Refresh all current categories
	for categoryName, mb := range ctx.categories {
		if err := mb.Refresh(); err != nil {
			// Backing file vanished: remove this category
			log.Printf("Removing category: %s: %s", categoryName, err)
			mb.Close()
			delete(ctx.categories, categoryName)
		}
	}

	// Any new categories?
	files, err := ioutil.ReadDir(ctx.MothballPath())
	if err != nil {
		log.Printf("Error listing mothballs: %s", err)
	}
	for _, f := range files {
		filename := f.Name()
		filepath := ctx.MothballPath(filename)
		if !strings.HasSuffix(filename, ".mb") {
			continue
		}
		categoryName := strings.TrimSuffix(filename, ".mb")

		if _, ok := ctx.categories[categoryName]; !ok {
			mb, err := OpenMothball(filepath)
			if err != nil {
				log.Printf("Error opening %s: %s", filepath, err)
				continue
			}
			log.Printf("New category: %s", filename)
			ctx.categories[categoryName] = mb
		}
	}
}

// readTeams reads in the list of team IDs,
// so we can quickly validate them.
func (ctx *Instance) readTeams() {
	filepath := ctx.StatePath("teamids.txt")
	teamids, err := os.Open(filepath)
	if err != nil {
		log.Printf("Error openining %s: %s", filepath, err)
		return
	}
	defer teamids.Close()

	// List out team IDs
	newList := map[string]bool{}
	scanner := bufio.NewScanner(teamids)
	for scanner.Scan() {
		teamId := scanner.Text()
		if (teamId == "..") || strings.ContainsAny(teamId, "/") {
			log.Printf("Dangerous team ID dropped: %s", teamId)
			continue
		}
		newList[scanner.Text()] = true
	}

	// For any new team IDs, set their next attempt time to right now
	now := time.Now()
	added := 0
	for k, _ := range newList {
		if _, ok := ctx.nextAttempt[k]; !ok {
			ctx.nextAttempt[k] = now
			added += 1
		}
	}

	// For any removed team IDs, remove them
	removed := 0
	for k, _ := range ctx.nextAttempt {
		if _, ok := newList[k]; !ok {
			delete(ctx.nextAttempt, k)
		}
	}

	if (added > 0) || (removed > 0) {
		log.Printf("Team IDs updated: %d added, %d removed", added, removed)
	}
}

// collectPoints gathers up files in points.new/ and appends their contents to points.log,
// removing each points.new/ file as it goes.
func (ctx *Instance) collectPoints() {
	logf, err := os.OpenFile(ctx.StatePath("points.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Can't append to points log: %s", err)
		return
	}
	defer logf.Close()

	files, err := ioutil.ReadDir(ctx.StatePath("points.new"))
	if err != nil {
		log.Printf("Error reading packages: %s", err)
	}
	for _, f := range files {
		filename := ctx.StatePath("points.new", f.Name())
		s, err := ioutil.ReadFile(filename)
		if err != nil {
			log.Printf("Can't read points file %s: %s", filename, err)
			continue
		}
		award, err := ParseAward(string(s))
		if err != nil {
			log.Printf("Can't parse award file %s: %s", filename, err)
			continue
		}

		duplicate := false
		for _, e := range ctx.PointsLog() {
			if award.Same(e) {
				duplicate = true
				break
			}
		}

		if duplicate {
			log.Printf("Skipping duplicate points: %s", award.String())
		} else {
			fmt.Fprintf(logf, "%s\n", award.String())
		}

		logf.Sync()
		if err := os.Remove(filename); err != nil {
			log.Printf("Unable to remove %s: %s", filename, err)
		}
	}
}

func (ctx *Instance) isEnabled() bool {
	// Skip if we've been disabled
	if _, err := os.Stat(ctx.StatePath("disabled")); err == nil {
		log.Print("Suspended: disabled file found")
		return false
	}

	untilspec, err := ioutil.ReadFile(ctx.StatePath("until"))
	if err == nil {
		untilspecs := strings.TrimSpace(string(untilspec))
		until, err := time.Parse(time.RFC3339, untilspecs)
		if err != nil {
			log.Printf("Suspended: Unparseable until date: %s", untilspec)
			return false
		}
		if until.Before(time.Now()) {
			log.Print("Suspended: until time reached, suspending maintenance")
			return false
		}
	}

	return true
}

// maintenance is the goroutine that runs a periodic maintenance task
func (ctx *Instance) Maintenance(maintenanceInterval time.Duration) {
	for {
		if ctx.isEnabled() {
			ctx.tidy()
			ctx.readTeams()
			ctx.collectPoints()
			ctx.generatePuzzleList()
			ctx.generatePointsLog()
		}
		select {
		case <-ctx.update:
			// log.Print("Forced update")
		case <-time.After(maintenanceInterval):
			// log.Print("Housekeeping...")
		}
	}
}
