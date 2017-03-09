
n=100
s=rep(NA,n)

s[1] = x[1]
s[2] = x[2]
b[2] = x[2] - x[1]
alpha = .1
for (t in 3:n) {
    s[t] = alpha*x[t] + (1-alpha)*(s[t-1] + b[t-1])
}


n=100
s=rep(NA,n)

s[1] = xx[1]
alpha = .1
for (t in 2:n) {
    s[t] = alpha*xx[t] + (1-alpha)*(s[t-1])
}


plot(xx,type="l",col="blue")
lines(s, col="red",type="l")

delta = 1.0 / 8.0
mu = 1.0
phi = 4.0
est = 0
dev = 0

getest <- function() {
 mu*est + phi*dev
}

y=rep(NA,n)
for (i in 1:n) {
    newSample = x[i]
    if (est == 0 ) {
        est = newSample
    }
	di = newSample - est
	est = est + delta * di
        di = abs(di)
        dev = dev + delta * (di - dev)
        y[i]=mu*est + phi*dev
}

r=rep(NA,n)
for (i in 1:n) { r[i]=addsam(a[i]);}

cat(addsam(a[i])); cat(", "); }
summary(a)

b= 300 + a
