package contact

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	graphql "github.com/graph-gophers/graphql-go"
	"github.com/rs/cors"
	"github.com/smithaitufe/go-graphql-upload"
)

var schemastring = `
	schema {
		query: Query
		mutation: Mutation
	}
	type Query {
		contacts: [Contact]!
	}
	type Mutation {
		createContact(input: ContactInput!): Contact
	}
	
	scalar Upload

	input ContactInput {
		firstName: String!
		lastName: String!
		photo: Upload!
	}

	type Profile {
		firstName: String!
		lastName: String!
	}
`

func StartAndListenGraphQL(port int) {
	schema := graphql.MustParseSchema(schemastring, &schemaResolver{})

	h := handler{Schema: schema}
	http.Handle("/graphiql", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "graphiql.html")
	}))
	http.Handle("/graphql", cors.Default().Handler(graphqlupload.Handler(h)))
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		log.Fatalf("could not start server. reason: %v", err)
	}

}

type handler struct {
	Schema *graphql.Schema
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var params struct {
		OperationName string                 `json:"operationName"`
		Variables     map[string]interface{} `json:"variables"`
		Query         string                 `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		fmt.Printf("bad request. %v", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	response := h.Schema.Exec(r.Context(), params.Query, params.OperationName, params.Variables)
	responseJSON, err := json.Marshal(response)
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(responseJSON)

}

type schemaResolver struct{}

type contactInput struct {
	FirstName string
	LastName  string
	Photo     graphqlupload.GraphQLUpload
}
type person struct {
	FirstName string
	LastName  string
}
type contactResolver struct {
	p person
}

func (r *contactResolver) FirstName() string {
	return r.p.FirstName
}
func (r *contactResolver) LastName() string {
	return r.p.LastName
}
func (r *schemaResolver) contacts(ctx context.Context) ([]*contactResolver, error) {
	persons := []person{
		person{FirstName: "Smith", LastName: "Samuel"},
		person{FirstName: "Friday", LastName: "Gabs"},
		person{FirstName: "Miriam", LastName: "Jude"},
		person{FirstName: "Stephen", LastName: "Stoke"},
		person{FirstName: "Rachael", LastName: "Magdalene"},
		person{FirstName: "Joseph", LastName: "Brown"},
		person{FirstName: "Sonia", LastName: "Fish"},
		person{FirstName: "Cynthia", LastName: "Gray"},
		person{FirstName: "Saint", LastName: "Rose"},
	}
	resolvers := make([]*contactResolver, len(persons))
	for k, v := range persons {
		resolvers[k] = &contactResolver{v}
	}
	return resolvers, nil
}
func (r *schemaResolver) CreateContact(ctx context.Context, args struct {
	Input contactInput
}) (*contactResolver, error) {
	// method 1: use the CreateReadStream to get a reader and manipulate it whichever way you want
	rd, _ := args.Input.Photo.CreateReadStream()
	b2, err := ioutil.ReadAll(rd)
	if err != nil {
		panic(err)
	}
	ioutil.WriteFile(args.Input.Photo.Filename, b2[:], 0666)

	// method 2: using WriteFile function. Easily write to any location in the local file system
	args.Input.Photo.WriteFile("/home/smith/Desktop/" + args.Input.Photo.Filename)
	return &contactResolver{person{FirstName: "Smith", LastName: "Samuel"}}, nil
}
