package main

import (
     _ "github.com/lib/pq"
    "github.com/joho/godotenv"
    "log"
    "fmt"
    "net/http"
    "strings"
    "sync/atomic"
    "encoding/json"
    "database/sql"
    "os"
    "time"
    "github.com/feagy/chirpy/internal/database"
    "github.com/feagy/chirpy/internal/auth"
    "github.com/google/uuid"
)

type apiConfig struct {
    fileserverHits atomic.Int32
    db *database.Queries
    jwtSecret string
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
    return  http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) handlerMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
    w.Write([]byte(fmt.Sprintf("<html><body><h1>Welcome, Chirpy Admin</h1><p>Chirpy has been visited %d times!</p></body></html>", cfg.fileserverHits.Load())))
}

func (cfg *apiConfig) handlerReset(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits.Store(0)
    cfg.db.DeleteUsers(r.Context())
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Hits reset to 0"))
}

type User struct {
	ID        uuid.UUID `json:"id"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
    Email     string `json:"email"`
}

func (cfg *apiConfig) handlerUsersPost(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
        Password string `json:"password"`
		Email string `json:"email"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't decode parameters", err)
		return
	}

    if params.Email == "" {
        respondWithError(w, http.StatusBadRequest, "Your email is empty be sure you use the true format for body", err)
        return
    }

    password, err := auth.HashPassword(params.Password)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error occured during password hash creation", err)
		return
	}

    user, err := cfg.db.CreateUser(r.Context(), database.CreateUserParams{Email: params.Email, HashedPassword: password})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create user", err)
		return
	}

    respondWithJSON(w, http.StatusCreated, User{
        ID: user.ID,
        CreatedAt: user.CreatedAt,
        UpdatedAt: user.UpdatedAt,
        Email: user.Email,
    })
}

func (cfg *apiConfig) handlerUsersLogin(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
        Password string `json:"password"`
		Email string `json:"email"`
        ExpiresInSeconds int `json:"expires_in_seconds"`
	}

    type response struct {
		User
		Token        string `json:"token"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't decode parameters", err)
		return
	}

    user, err := cfg.db.GetUserByEmail(r.Context(), params.Email)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Incorrect email or password", err)
		return
	}

    match, err := auth.CheckPasswordHash(params.Password, user.HashedPassword)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Incorrect email or password", err)
		return
	}
        
    if match == false {
        respondWithError(w, http.StatusUnauthorized, "Incorrect email or password", err)
        return
    }

	expirationTime := time.Hour
	if params.ExpiresInSeconds > 0 && params.ExpiresInSeconds < 3600 {
		expirationTime = time.Duration(params.ExpiresInSeconds) * time.Second
	}

	accessToken, err := auth.MakeJWT(
		user.ID,
		cfg.jwtSecret,
		expirationTime,
	)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create access JWT", err)
		return
	}

    respondWithJSON(w, http.StatusOK, response{
        User:User{
            ID: user.ID,
            CreatedAt: user.CreatedAt,
            UpdatedAt: user.UpdatedAt,
            Email: user.Email,
        },
        Token: accessToken})
}

type Chirp struct {
    ID uuid.UUID `json:"id"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
    Body      string `json:"body"`
    UserID  uuid.UUID  `json:"user_id"`
}

func (cfg *apiConfig) handlerChirpsPost(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body string `json:"body"`
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}
	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err = decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't decode parameters", err)
		return
	}

	const maxChirpLength = 140
	if len(params.Body) > maxChirpLength {
		respondWithError(w, http.StatusBadRequest, "Chirp is too long", nil)
		return
	}

    bannedWords := []string{"kerfuffle", "sharbert", "fornax"}
    words := strings.Split(params.Body, " ")
    for _, banned := range bannedWords {
        for i, word := range words {
            if strings.ToLower(word) == banned {
                words[i] = "****"  
            }        
        }
    }

    chirp, err := cfg.db.CreateChirp(r.Context(), database.CreateChirpParams{
        Body: strings.Join(words, " "), 
        UserID: userID})

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error occured during chirp creation", err)
		return
	}
    
	respondWithJSON(w, http.StatusCreated, Chirp{
        ID: chirp.ID,
        CreatedAt: chirp.CreatedAt,
        UpdatedAt: chirp.UpdatedAt,
		Body: chirp.Body,
        UserID: chirp.UserID,
	})
}

func (cfg *apiConfig) handlerChirpsGet(w http.ResponseWriter, r *http.Request) {
    chirps, err := cfg.db.GetAllChirps(r.Context())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error occured during chirp retrieval", err)
		return
	}

    returnChirps := []Chirp{}
    
	for _, chirp := range chirps {
		returnChirps = append(returnChirps, Chirp{
			ID:        chirp.ID,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			Body:      chirp.Body,
			UserID:    chirp.UserID,
		})
	}

	respondWithJSON(w, http.StatusOK, returnChirps)
}

func (cfg *apiConfig) handlerChirpsGetById(w http.ResponseWriter, r *http.Request) {
	chirpIdString := r.PathValue("chirpId")
	chirpId, err := uuid.Parse(chirpIdString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid chirp Id", err)
		return
	}

    chirp, err := cfg.db.GetChirpById(r.Context(), chirpId)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Given id not found", err)
		return
	}

	respondWithJSON(w, http.StatusOK, Chirp{
		ID:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserID:    chirp.UserID,
	})
}


func main(){
    godotenv.Load()
    dbURL := os.Getenv("DB_URL")
    jwtSecret := os.Getenv("JWT_SECRET")

	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is not set")
	}
    
    
    db, err := sql.Open("postgres", dbURL)
    if err != nil {
        log.Fatal("An error occured during database connection")
    }
    dbQueries := database.New(db)
  	const filepathRoot = "."
	const port = "8080"

    apiCfg := apiConfig{
        fileserverHits:atomic.Int32{},
        db:dbQueries,
        jwtSecret:jwtSecret,}
	mux := http.NewServeMux()

	mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(filepathRoot)))))
	
    mux.HandleFunc("GET /api/healthz", handlerReadiness)
    mux.HandleFunc("POST /api/chirps", apiCfg.handlerChirpsPost)
    mux.HandleFunc("GET /api/chirps", apiCfg.handlerChirpsGet)
    mux.HandleFunc("GET /api/chirps/{chirpId}", apiCfg.handlerChirpsGetById)
    mux.HandleFunc("POST /api/users", apiCfg.handlerUsersPost)
    mux.HandleFunc("POST /api/login", apiCfg.handlerUsersLogin)
    
    mux.HandleFunc("GET /admin/metrics", apiCfg.handlerMetrics)
    mux.HandleFunc("POST /admin/reset", apiCfg.handlerReset)


	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	log.Printf("Serving files from %s on port: %s\n", filepathRoot, port)
	log.Fatal(srv.ListenAndServe())
}

func handlerReadiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(http.StatusText(http.StatusOK)))
}

func respondWithError(w http.ResponseWriter, code int, msg string, err error) {
	if err != nil {
		log.Println(err)
	}
	if code > 499 {
		log.Printf("Responding with 5XX error: %s", msg)
	}
	type errorResponse struct {
		Error string `json:"error"`
	}
	respondWithJSON(w, code, errorResponse{
		Error: msg,
	})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	dat, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.WriteHeader(code)
	w.Write(dat)
}
