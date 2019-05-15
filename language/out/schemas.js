const {ApolloServer, makeExecutableSchema} = require("apollo-server");

const rootSchema = `
    type Query {
        foo: String
    }
    
    directive @foo on SCHEMA | UNION
    
    interface Fooer {
        foo: String
    }
    
    type Mut implements Fooer {
        foo: String
    }
    
    extend type Mut implements Fooer {
        bar: String
    }
    
    union Foo
`;

const schema = makeExecutableSchema({
    typeDefs: [rootSchema],
    resolvers: {}
});

const server = new ApolloServer({schema});

server.listen().then(({url}) => {
    console.log(`Server running at ${url}`)
});
