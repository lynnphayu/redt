package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/gin-gonic/gin"
)

var routeCollection *mongo.Collection
var accessLogCollection *mongo.Collection

var ctx = context.TODO()

type Route struct {
	ID        primitive.ObjectID `bson:"_id"`
	CreatedAt time.Time          `bson:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt"`
	Name      string             `bson:"name"`
	Route     string             `bson:"route"`
	Path      string             `bson:"path"`
}

type AccessLog struct {
	ID             primitive.ObjectID `bson:"_id"`
	CreatedAt      time.Time          `bson:"createdAt"`
	UpdatedAt      time.Time          `bson:"updatedAt"`
	Path           string             `bson:"path"`
	AccessOriginIP string             `bson:"accessOriginIP"`
	HTTPMethod     string             `bson:"HTTPMethod"`
	Referer        string             `bson:"referer"`
	Status         string             `bson:"status"`
}

func init() {
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017/")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatal(err)
	}
	routeCollection = client.Database("test").Collection("routes")
	accessLogCollection = client.Database("test").Collection("accesslogs")
}

func getDocs(collection *mongo.Collection, filter interface{}) ([]*Route, error) {
	var routes []*Route

	cur, err := collection.Find(ctx, filter)
	if err != nil {
		return routes, err
	}
	for cur.Next(ctx) {
		var r Route
		err := cur.Decode(&r)
		if err != nil {
			return routes, err
		}
		routes = append(routes, &r)
	}
	if cur.Err(); err != nil {
		return routes, err
	}
	cur.Close(ctx)
	if len(routes) == 0 {
		return routes, mongo.ErrNoDocuments
	}
	return routes, nil
}

func getRedirectLocations(path string) (*Route, error) {
	var route *Route
	filter := bson.M{"path": path}
	routes, err := getDocs(routeCollection, filter)
	if err != nil {
		return route, err
	}
	return routes[0], nil
}

func createAccessLogFromHTTPRequest(httpRequest *http.Request, status string) {
	accessOriginIP := httpRequest.RemoteAddr
	httpMethod := httpRequest.Method
	referer := httpRequest.Referer()

	_, err := accessLogCollection.InsertOne(ctx, &AccessLog{
		ID:             primitive.NewObjectID(),
		AccessOriginIP: accessOriginIP,
		HTTPMethod:     httpMethod,
		Referer:        referer,
		Status:         status,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	})
	if err != nil {
		log.Fatal(err)
	}
}

// func Logger() gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		t := time.Now()

// 		// Set example variable
// 		println(c.Request.RequestURI)
// 		// before request
// 		c.Next()

// 		// after request
// 		latency := time.Since(t)
// 		log.Print(latency)

// 		// access the status we are sending
// 		status := c.Writer.Status()
// 	}
// }

func main() {
	router := gin.Default()

	// router.Use(Logger())

	router.GET("/r/:path", func(c *gin.Context) {
		// extract path from url
		path := c.Param("path")

		// extract query `q` from query-string
		query := c.Query("q")

		// get redirected location
		route, err := getRedirectLocations(path)
		if route != nil {
			println(route.Route + query)
		}
		if err != nil && err == mongo.ErrNoDocuments {
			// create access log for redirection for redirection failing
			go createAccessLogFromHTTPRequest(c.Request, "not-found")
			c.String(http.StatusOK, "No Redirect Found!.")
		} else {
			// create access log for redirection for successful redirection
			go createAccessLogFromHTTPRequest(c.Request, "success")
			c.Redirect(http.StatusPermanentRedirect, route.Route+query)
		}
	})
	router.Run()
}
