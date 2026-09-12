# API

- [Public](#public)
- [Signup / login](#signup--login)
- [Collections](#collections)
- [Collection colors](#collection-colors)
- [Export](#export)
- [More](#more)

The spec - [api/api.yaml](../api/api.yaml).

Sign up, then log in.
A user's data (collection, colors) need the token from the login response.

```sh
HOST=localhost:8080
```

## Public

```sh
# public CSS palette
curl $HOST/api/v1/colors
curl "$HOST/api/v1/colors?sort=hex&order=desc"
# harmonies: complement, triad, ramp, and others
curl $HOST/api/v1/colors/ff00aa/complement
curl $HOST/api/v1/colors/ff00aa/triad
curl $HOST/api/v1/version
```

## Signup / login

```sh
# signup
curl -X POST $HOST/api/v1/users -d '{"nickname":"alice","password":"correct-horse"}'
# {"user":{"nickname":"alice", "created_at":"..."}}

# login
curl -X POST $HOST/api/v1/tokens -d '{"nickname":"alice","password":"correct-horse"}'
# 201 {"token":"...","expiry":"..."}

TOKEN="<token from above>"

# who am i
curl $HOST/api/v1/me -H "Authorization: Bearer $TOKEN"
# {"user":{"nickname":"alice", "created_at":"..."}}

# log out
curl -X DELETE $HOST/api/v1/tokens -H "Authorization: Bearer $TOKEN"
```

## Collections

```sh
# default Main collection is auto created
curl $HOST/api/v1/me/collections -H "Authorization: Bearer $TOKEN"
# {"collections":[{"id":<uuid>, "name":"Main", "is_default":true, ...}]}

# add collection
curl -X POST $HOST/api/v1/me/collections -H "Authorization: Bearer $TOKEN" -d '{"name":"Sun"}'
# {"collection":{"id":<uuid>, "name":"Sun", ...}}

CLT="<collection id from above>"

# delete collection and all its colors (the default collection can't be deleted)
curl -X DELETE $HOST/api/v1/me/collections/$CLT -H "Authorization: Bearer $TOKEN"
```

## Collection colors

```sh
CLT="<collection id from above>"

# add color
curl -X POST $HOST/api/v1/me/collections/$CLT/colors -H "Authorization: Bearer $TOKEN" -d '{"hex":"FF00AA"}'

# list collection colors
curl $HOST/api/v1/me/collections/$CLT/colors -H "Authorization: Bearer $TOKEN"
# {"colors":[{"hex":"#ff00aa","created_at":"..."}],"metadata":{"limit":50}}

# delete color
curl -X DELETE $HOST/api/v1/me/collections/$CLT/colors/ff00aa -H "Authorization: Bearer $TOKEN"
```

## Export

```sh
# export account data as a JSON file
curl -OJ $HOST/api/v1/me/export -H "Authorization: Bearer $TOKEN"
# downloading file named like "wavelen-alice-20260912T193219Z.json"
```

## More

Not all endpoints and their options are covered here, see the spec.
