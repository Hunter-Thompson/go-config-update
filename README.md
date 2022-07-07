# go-config-update 

## Usage

```sh
$ export GIT_TOKEN=ghp_asdhaksdj
$ ./go-config-update --imageid v0.0.1 \
                 --reponame test \
                 --repoclone test-repo \
                 --custom true \
                 --imageprefix 12837234.dkr.ecr.us-east-1.amazonaws.com \
                 --configfolder apps/test \
                 --vipersearch "config.image,config.version" \
                 --githuborg test \
                 --configtype json \
                 --confignames "test.json,test2.json" \
                 --commitmessage helloworld \
                 --headbranchname master \
                 --githubusername test \
                 --githubemail asd@test.com 

{"level":"info","time":"2022-06-02T14:18:54+05:30","message":"updating image v0.0.1 for test by cloning test-repo and searching for config.image"}
```
