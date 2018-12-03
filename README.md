# psql-rls

## TL;DR

- PostgreSQLのRow Level Security機能をアプリケーションで使う検証
- サンプルとして Tenant, Productのドメインモデルを用意
- Tenantごとにデータベースユーザーを作成, RLSを設定
- 各Tenantが自身に関係するProductのみしか検索/更新/削除できないことがRLSによって保証されるかを確認
- 上記をやる上で、DSNをユーザーの数だけ作って接続する必要があるが、その辺の実装どうするかも考える

## 作業ログ

Connect database

```console
psql -U xuser -d xdb
```

操作いろいろ:
https://dev.classmethod.jp/server-side/db/postgresql-organize-command/

Create database user

```console
createuser --connection-limit=10
```

Table schema

```console
xdb=# \d products
                                       Table "public.products"
   Column   |           Type           | Collation | Nullable |               Default
------------+--------------------------+-----------+----------+--------------------------------------
 id         | integer                  |           | not null | nextval('products_id_seq'::regclass)
 created_at | timestamp with time zone |           |          |
 updated_at | timestamp with time zone |           |          |
 deleted_at | timestamp with time zone |           |          |
 tenant_id  | integer                  |           |          |
 title      | text                     |           |          |
 price      | integer                  |           |          |
Indexes:
    "products_pkey" PRIMARY KEY, btree (id)
    "idx_products_deleted_at" btree (deleted_at)
```

Table access policy

```console
xdb-# \z products
                              Access privileges
 Schema |   Name   | Type  | Access privileges | Column privileges | Policies
--------+----------+-------+-------------------+-------------------+----------
 public | products | table |                   |                   |
(1 row)
```

Policy table

```console
xdb=# select * from pg_policy;
 polname | polrelid | polcmd | polpermissive | polroles | polqual | polwithcheck
---------+----------+--------+---------------+----------+---------+--------------
(0 rows)
```
