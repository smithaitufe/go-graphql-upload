# go-graphqlupload

A file upload middleware for gophers. It enables file upload operations to be done in [Go] (https://github.com/google/) using [apollo-upload-client] (https://github.com/jaydenseric/apollo-upload-client) and [graph-gophers/graphql-go] (https://github.com/graph-gophers/graphql-go) packages.


## Usage

Add the line below to the list of imports

`github.com/smithaitufe/go-graphqlupload`

Pass the graphql endpoint handler to the `graphqlupload.Handler` middleware function.
Eg.

`http.Handler("/graphql", graphqlupload.Handler(relay.Handler...))`

In your input struct declaration, simply use the `graphqlupload.GraphQLUpload` type as datatype against the file input variable.

Eg.

```og
type contactInput struct {
    photo graphqlupload.GraphQLUpload
}
```
In the schema definition, add the following  `scalar Upload` and you are good to go.


The `graphqlupload.GraphQLUpload` has two methods

`CreateReadStream` returns a reader to the upload file(s)
`WriteFile` accepts a `name` string that indicates where the file should be saved into

In the example folder, there is a working sample.

Thanks.

I hope to improve on it in the coming days

