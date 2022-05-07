# ifacecapture

Go linter that tries to find possibly unintentional usage of captured variables
implementing an inner-scoped parameter's interface.

This linter was originally designed to try and find mistakes like the following
in code implementing database transactions:

```go
type Database interface {
    Transaction(tx Database)
    DoQuery()
}

func doDatabaseStuff(db Database) {
    db.Transaction(func(tx Database) { // tx is a transaction handle
        db.DoQuery() 
        // the programmer probably meant to use the transaction handle "tx", but
        // accidentally used the outer scope variable "db"
    })
}
```

It searches for functions taking a callback, which itself takes one or 
parameters which are of some interface type. It then searches for captures of
outer scope variables which implement that interface.

This linter may help you find similar patterns, where you have a callback
function intended to aid safe usage of an interface. Unintentional captures 
of an outer scope variable that also implements that interface often means
that there was a mistake.

## TODO:

- Logging and diagnostics
- More tests
- Check multiple interface parameters
- Cleaner code
- Configurability
- False positive/negative instead of crashing
