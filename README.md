This linter was originally designed to try and find mistakes like the following
in code implementing database transactions:

```go
type Database interface {
    Transaction(tx Database)
    DoQuery()
}

func doDatabaseStuff(db Database) {
    db.Transaction(func(tx Database) {
        db.DoQuery() 
        // the programmer probably meant to use the transaction handle, but
        // accidentally used the outer scope variable
    })
}
```

It searches for functions taking a callback, which itself takes one or 
parameters which are of some interface type. It then searches for captures of
outer scope variables which implement that interface.