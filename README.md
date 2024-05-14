## What is this? 

Simple Shell For Run docker image inspired from gitlab ci 

## How to use it?

create configurationfile in yml format like this

```yml
cache:
  paths:
    - /node_modules
test: 
  image: node:20-slim
  before_script:
    - npm ci --legacy-peer-deps
  script:
    - npm run test
```

and run the command 

```bash
go run main.go -c config.yml <path for project>
```

## How to build it?

```bash
go build -o builder .
```
