package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	host     = "postgres-postgresql.api.svc.cluster.local"
	port     = 5432
	user     = "api"
	password = "123"
	dbname   = "mydb"
)

// const (
// 	host     = "localhost"
// 	port     = 5432
// 	user     = "146850"
// 	password = "123"
// 	dbname   = "mydb"
// )

var db *sql.DB

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func getUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, name, email FROM users")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		err := rows.Scan(&user.ID, &user.Name, &user.Email)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		users = append(users, user)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Service up")
}

func addUser(w http.ResponseWriter, r *http.Request) {
	var user User
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err = db.Exec("INSERT INTO users (name, email) VALUES ($1, $2)", user.Name, user.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, "User added successfully")
}

func updateUser(w http.ResponseWriter, r *http.Request) {
	var user User
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err = db.Exec("UPDATE users SET name=$1, email=$2 WHERE id=$3", user.Name, user.Email, user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "User updated successfully")
}

func deleteUser(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "ID parameter is required", http.StatusBadRequest)
		return
	}

	_, err := db.Exec("DELETE FROM users WHERE id=$1", id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "User deleted successfully")
}

// ////
func init() {
	_ = godotenv.Load()
}

var httpRequestCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "http_requests_total",
	Help: "Total number of HTTP requests received",
}, []string{"status", "path", "method"})

var activeRequestsGauge = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "http_active_requests",
		Help: "Number of active connections to the service",
	},
)

// Helper to capture HTTP status codes
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.statusCode = code
	rec.ResponseWriter.WriteHeader(code)
}

// Middleware to count HTTP requests
func prometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Wrap the ResponseWriter to capture the status code
		activeRequestsGauge.Inc()
		recorder := &statusRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		time.Sleep(2 * time.Second)

		// Process the request
		next.ServeHTTP(recorder, r)

		activeRequestsGauge.Dec()

		method := r.Method
		path := r.URL.Path // Path can be adjusted for aggregation (e.g., `/users/:id` → `/users/{id}`)
		status := strconv.Itoa(recorder.statusCode)

		// Increment the counter
		httpRequestCounter.WithLabelValues(status, path, method).Inc()
	})
}

func main() {
	pgConnStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, dbname)

	conn, err := sql.Open("postgres", pgConnStr)
	if err != nil {
		log.Fatalf("Error opening database connection: %v", err)
	}
	db = conn
	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Fatalf("Error connecting to the database: %v", err)
	}
	fmt.Println("Connected to the PostgreSQL database")

	mux := http.NewServeMux()

	reg := prometheus.NewRegistry()

	reg.MustRegister(httpRequestCounter)
	reg.MustRegister(activeRequestsGauge)

	handler := promhttp.HandlerFor(
		reg,
		promhttp.HandlerOpts{})

	mux.Handle("/metrics", handler)

	mux.HandleFunc("/users", getUsers)
	mux.HandleFunc("/users/add", addUser)
	mux.HandleFunc("/users/update", updateUser)
	mux.HandleFunc("/users/delete", deleteUser)
	mux.HandleFunc("/health", healthCheck)

	promHandler := prometheusMiddleware(mux)

	fmt.Println("Server is listening on port 8080 test")
	if err := http.ListenAndServe(":8080", promHandler); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
