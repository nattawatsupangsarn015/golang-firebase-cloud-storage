package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	firebase "firebase.google.com/go"
	guuid "github.com/google/uuid"
	"golang-firebase-cloud-storage/models"

	"cloud.google.com/go/firestore"
	cloud "cloud.google.com/go/storage"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
)

type App struct {
	Router  *mux.Router
	ctx     context.Context
	client  *firestore.Client
	storage *cloud.Client
}

func main() {
	godotenv.Load()
	route := App{}
	route.Init()
	route.Run()
}

func (route *App) Init() {

	route.ctx = context.Background()

	sa := option.WithCredentialsFile("serviceAccountKey.json")

	var err error

	app, err := firebase.NewApp(route.ctx, nil, sa)
	if err != nil {
		log.Fatalln(err)
	}

	route.client, err = app.Firestore(route.ctx)
	if err != nil {
		log.Fatalln(err)
	}

	route.storage, err = cloud.NewClient(route.ctx, sa)
	if err != nil {
		log.Fatalln(err)
	}

	route.Router = mux.NewRouter()
	route.initializeRoutes()
	fmt.Println("Successfully connected at port : " + route.GetPort())
}

func (route *App) GetPort() string {
	var port = os.Getenv("MyPort")
	if port == "" {
		port = "5000"
	}
	return ":" + port
}

func (route *App) Run() {
	log.Fatal(http.ListenAndServe(route.GetPort(), route.Router))
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func (route *App) initializeRoutes() {
	route.Router.HandleFunc("/", route.Home).Methods("GET")
	route.Router.HandleFunc("/upload/image", route.UploadImage).Methods("POST")
}

func (route *App) Home(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusOK, "Hello World!")
}

func (route *App) UploadImage(w http.ResponseWriter, r *http.Request) {

	file, handler, err := r.FormFile("image")
	r.ParseMultipartForm(10 << 20)
	if err != nil {
		respondWithJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	defer file.Close()

	imagePath := handler.Filename

	bucket := "golang-cloud-firestore.appspot.com"

	wc := route.storage.Bucket(bucket).Object(imagePath).NewWriter(route.ctx)
	uuid := guuid.New()
	wc.Metadata = map[string]string{
		"firebaseStorageDownloadTokens": uuid.String(),
	}
	_, err = io.Copy(wc, file)
	if err != nil {
		respondWithJSON(w, http.StatusBadRequest, err.Error())
		return

	}
	if err := wc.Close(); err != nil {
		respondWithJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	err = CreateImageUrl(imagePath, bucket, route.ctx, route.client, uuid)
	if err != nil {
		respondWithJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	respondWithJSON(w, http.StatusCreated, "Create image success.")
}

func CreateImageUrl(imagePath string, bucket string, ctx context.Context, client *firestore.Client, uuid guuid.UUID) error {
	imageStructure := models.ImageStructure{
		ImageName: imagePath,
		URL:       fmt.Sprintf("https://firebasestorage.googleapis.com/v0/b/%s/o/%s?alt=media&token=%s", bucket, imagePath, uuid.String()),
	}

	_, _, err := client.Collection("image").Add(ctx, imageStructure)
	if err != nil {
		return err
	}

	return nil
}
