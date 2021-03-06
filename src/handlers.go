package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"os/exec"
	"path"
)

// https://github.com/omniti-labs/jsend
type JSend struct {
	Status string `json:"status"`
	Data   struct {
		Short       string `json:"short"`
		Description string `json:"description"`
	} `json:"data"`
}

const (
	JSendSuccess = "success"
	JSendFail    = "fail"
	JSendError   = "error"
)

func respond(w http.ResponseWriter, req *http.Request, status string, short string, format string, a ...interface{}) {
	resp := JSend{}
	resp.Status = status
	resp.Data.Short = short
	resp.Data.Description = fmt.Sprintf(format, a...)

	respBytes, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // RFC2616 makes it pretty clear that 4xx codes are for the user-agent
	w.Write(respBytes)
}

// hasLine returns true if line appears in r.
// The entire line must match.
func hasLine(r io.Reader, line string) bool {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if scanner.Text() == line {
			return true
		}
	}
	return false
}

// hasSubstr returns the line where substr appears in r.
// The line must contain substr.
func hasSubstr(r io.Reader, substr string) string {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), substr) {
			return scanner.Text()
		}
	}
	return ""
}

func (ctx *Instance) registerHandler(w http.ResponseWriter, req *http.Request) {
	teamName := req.FormValue("name")
	teamId := req.FormValue("id")

	// Keep foolish operators from shooting themselves in the foot
	// You would have to add a pathname to your list of Team IDs to open this vulnerability,
	// but I have learned not to overestimate people.
	if strings.Contains(teamId, "../") {
		teamId = "rodney"
	}

	if (teamId == "") || (teamName == "") {
		respond(
			w, req, JSendFail,
			"Invalid Entry",
			"Either `id` or `name` was missing from this request.",
		)
		return
	}

	teamIds, err := os.Open(ctx.StatePath("teamids.txt"))
	if err != nil {
		respond(
			w, req, JSendFail,
			"Cannot read valid team IDs",
			"An error was encountered trying to read valid teams IDs: %v", err,
		)
		return
	}
	defer teamIds.Close()
	if !hasLine(teamIds, teamId) {
		respond(
			w, req, JSendFail,
			"Invalid Team ID",
			"I don't have a record of that team ID. Maybe you used capital letters accidentally?",
		)
		return
	}

	f, err := os.OpenFile(ctx.StatePath("teams", teamId), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsExist(err) {
			respond(
				w, req, JSendFail,
				"Already registered",
				"This team ID has already been registered.",
			)
		} else {
			log.Print(err)
			respond(
				w, req, JSendFail,
				"Registration failed",
				"Unable to register. Perhaps a teammate has already registered?",
			)
		}
		return
	}
	defer f.Close()

	fmt.Fprintln(f, teamName)
	respond(
		w, req, JSendSuccess,
		"Team registered",
		"Your team has been named and you may begin using your team ID!",
	)
}

func (ctx *Instance) answerHandler(w http.ResponseWriter, req *http.Request) {
	teamId := req.FormValue("id")
	category := req.FormValue("cat")
	pointstr := req.FormValue("points")
	answer := req.FormValue("answer")
	//log.Print("Got ", answer)

	if ! ctx.ValidTeamId(teamId) {
		respond(
			w, req, JSendFail,
			"Invalid team ID",
			"That team ID is not valid for this event.",
		)
		return
	}
	if ctx.TooFast(teamId) {
		respond(
			w, req, JSendFail,
			"Submitting too quickly",
			"Your team can only submit one answer every %v", ctx.AttemptInterval,
		)
		return
	}

	points, err := strconv.Atoi(pointstr)
	if err != nil {
		respond(
			w, req, JSendFail,
			"Cannot parse point value",
			"This doesn't look like an integer: %s", pointstr,
		)
		return
	}
	
	if ctx.options["progression"] == "team" {
		_, ok := ctx.unlockedPuzzles[teamId][category][points]
		if !ok {
			respond(
				w, req, JSendFail,
				"Puzzle locked",
				"Your team has to unlock that puzzle first",
			)
			return
		}
	}

	haystack, err := ctx.OpenCategoryFile(category, "answers.txt")
	foundAns := false
	if err != nil {
		// We did not find an answer file, but we can still look for a dynamic answer
	} else {
		// Look for the answer
		needle := fmt.Sprintf("%d %s", points, answer)
		if !hasLine(haystack, needle) {
			// We did not find the answer, but we can still look for a dynamic answer
		} else {
			foundAns = true
		}
	}
	defer haystack.Close()

	
	if !foundAns {
		// Now we look for a dynamic answer, since no static one matched
		haystackdyn, errdyn := ctx.OpenCategoryFile(category, "answerdyn.txt")
		if errdyn != nil && err != nil {
			respond(
				w, req, JSendFail,
				"Cannot list answers",
				"Unable to read the list of static or dynamic answers for this category.",
			)
			return
		}
		defer haystackdyn.Close()

		// Look for the answerdyn file
		needledyn := fmt.Sprintf("%d", points)
		answerFile := hasSubstr(haystackdyn, needledyn)
		if answerFile == "" {
			// This is where the answer file is run, check to make sure needledyn is the full line
			// If this code is reached, neither answers nor answersdyn has an entry for the submission.
			respond(
				w, req, JSendFail,
				"Wrong answer",
				"That is not the correct answer for %s %d.", category, points,
			)
			return
		} else {
			//If this point is reached, then we have a dynamic grader to run
			splitAnswer := strings.Split(answerFile + " " + answer, " ")
			recombinedCommand := splitAnswer[1]
			splitAnswer = splitAnswer[2:]
			cmd := exec.Command(recombinedCommand, splitAnswer...)
			
			// Now we read the map to get the correct answer file directory
			haystackmap, errmap := ctx.OpenCategoryFile(category, "map.txt")
			if errmap != nil && err != nil {
				respond(
					w, req, JSendFail,
					"Cannot read map",
					"Unable to read the map for the category.",
				)
				return
			}
			defer haystackmap.Close()
			mappedDir := hasSubstr(haystackmap, needledyn)
			splitMap := strings.Split(mappedDir, " ")
			gradeDir, direrr := ctx.GetExtractedCategoryDir(category)
			cmd.Dir = path.Join(gradeDir, "answerdyn", splitMap[1])
			
			// Now we run the dynamic grader command with the answer as the first argument
			out, cmderr := cmd.CombinedOutput()
			theOutput := string(out)
			theOutput = strings.TrimRight(theOutput, "\n")
			if cmderr != nil || direrr != nil || theOutput != "true" {
				// The answer script must return "true" on stdout to be correct
				respond(
					w, req, JSendFail,
					"Wrong answer",
					"That is not the correct answer for %s %d.", category, points,
				)
				return
			}
		}
	}

	if err := ctx.AwardPoints(teamId, category, points); err != nil {
		respond(
			w, req, JSendError,
			"Cannot award points",
			"The answer is correct, but there was an error awarding points: %v", err.Error(),
		)
		return
	}
	respond(
		w, req, JSendSuccess,
		"Points awarded",
		fmt.Sprintf("%d points for %s!", points, teamId),
	)
}

func (ctx *Instance) puzzlesHandler(w http.ResponseWriter, req *http.Request) {
	teamId := req.FormValue("id")
	if _, err := ctx.TeamName(teamId); err != nil {
		http.Error(w, "Must provide team ID", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if ctx.options["progression"] == "team" {
		w.Write(ctx.jPuzzleListTeam[teamId])
	} else {
		w.Write(ctx.jPuzzleList)
	}
}

func (ctx *Instance) pointsHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(ctx.jPointsLog)
}

func (ctx *Instance) contentHandler(w http.ResponseWriter, req *http.Request) {
	// Prevent directory traversal
	if strings.Contains(req.URL.Path, "/.") {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	// Be clever: use only the last three parts of the path. This may prove to be a bad idea.
	parts := strings.Split(req.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	fileName := parts[len(parts)-1]
	puzzleId := parts[len(parts)-2]
	categoryName := parts[len(parts)-3]

	mb, ok := ctx.categories[categoryName]
	if !ok {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	mbFilename := fmt.Sprintf("content/%s/%s", puzzleId, fileName)
	mf, err := mb.Open(mbFilename)
	if err != nil {
		log.Print(err)
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	defer mf.Close()

	http.ServeContent(w, req, fileName, mf.ModTime(), mf)
}

func (ctx *Instance) staticHandler(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	if strings.Contains(path, "..") {
		http.Error(w, "Invalid URL path", http.StatusBadRequest)
		return
	}
	if path == "/" {
		path = "/index.html"
	}

	f, err := os.Open(ctx.ThemePath(path))
	if err != nil {
		http.NotFound(w, req)
		return
	}
	defer f.Close()

	d, err := f.Stat()
	if err != nil {
		http.NotFound(w, req)
		return
	}

	http.ServeContent(w, req, path, d.ModTime(), f)
}

type FurtiveResponseWriter struct {
	w          http.ResponseWriter
	statusCode *int
}

func (w FurtiveResponseWriter) WriteHeader(statusCode int) {
	*w.statusCode = statusCode
	w.w.WriteHeader(statusCode)
}

func (w FurtiveResponseWriter) Write(buf []byte) (n int, err error) {
	n, err = w.w.Write(buf)
	return
}

func (w FurtiveResponseWriter) Header() http.Header {
	return w.w.Header()
}

// This gives Instances the signature of http.Handler
func (ctx *Instance) ServeHTTP(wOrig http.ResponseWriter, r *http.Request) {
	w := FurtiveResponseWriter{
		w:          wOrig,
		statusCode: new(int),
	}
	ctx.mux.ServeHTTP(w, r)
	log.Printf(
		"%s %s %s %d\n",
		r.RemoteAddr,
		r.Method,
		r.URL,
		*w.statusCode,
	)
}

func (ctx *Instance) BindHandlers() {
	ctx.mux.HandleFunc(ctx.Base+"/", ctx.staticHandler)
	ctx.mux.HandleFunc(ctx.Base+"/register", ctx.registerHandler)
	ctx.mux.HandleFunc(ctx.Base+"/answer", ctx.answerHandler)
	ctx.mux.HandleFunc(ctx.Base+"/content/", ctx.contentHandler)
	ctx.mux.HandleFunc(ctx.Base+"/puzzles.json", ctx.puzzlesHandler)
	ctx.mux.HandleFunc(ctx.Base+"/points.json", ctx.pointsHandler)
}
