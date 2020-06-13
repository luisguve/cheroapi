# gRPC Server for the cheroAPI service

This repository contains the implementation for the CrudCheropatilla service, as defined in [cheroapi.proto](/internal/proto/cheroapi.proto), using [Bolt](https://github.com/etcd-io/bbolt) as the underlying database management system.

### An overview of ***cheropatilla***

***Cheropatilla*** is a social platform that introduces its own idea of how pagination of contents should be done. In order to understand it, we should first review what's the current way in which some of the most popular social networks out there achieve pagination of millions of posts every day, for every user.

1. **Twitter**. The main contents here are the so-called *tweets* and *re-tweets*; you can post an unlimited number of tweets no longer than 280 characters, attach up to four pictures or a video, and your followers and the followers of every user that *re-tweet* your tweets, will see them in their timeline, **in chronological order**. Additionally, you can use *#hashtags* to 

2. **Reddit**. This is actually a platform that enables users to discuss several specific topics
