# psql-rls

## TL;DR

- PostgreSQLのRow Level Security機能をアプリケーションで使う検証
- 用途としてはマルチテナントの管理に使ってみる
- コードサンプルとして Tenant, Productのドメインモデルを用意
- Tenantごとにデータベースユーザーを作成, RLSを設定
- 各Tenantが自身に関係するProductのみしか検索/更新/削除できないことがRLSによって保証されるかを確認
- 上記をやる上で、DSNをユーザーの数だけ作って接続する必要があるが、その辺の実装どうするかも考える

コードを書く前に、まずはREPLでpotgre動きとかRLSの使い方を確認していく。

## Postgres

最新バージョン `11.x`

documentation: https://www.postgresql.org/docs/11/index.html

## 作業ログ

### とりあえずpostgreSQLを触る

Connect database

databaseはdocker-composeで作ってる。

```
psql -U xuser -d xdb
```

インタラクティブにパスワード入力させられる。

Show tables

```
\dt
```

Table schema

```
\d table
```

show access policy of table

```
\z table
```

Get current user

```
select current_user;
```

Show user's role

```
\du
```

> xuser     | Superuser, Create role, Create DB, Replication, Bypass RLS | {}

super userで実行すると上みたいな感じ。ちなみにスーパーユーザには全てのRLSを許可される権限がついている `Bypass RLS`

All users

```
select * from pg_user;
```

Add role to user

```
grant all on table to user;
```

Switch user

```
\connect - <USER_NAME>
```

Table access policy

```
\z products
```

Policy table

```
select * from pg_policy;
```

### RLSを使ってみる

About RLS: https://www.postgresql.org/docs/current/ddl-rowsecurity.html

テーブルのRLSをenableにすると、アクセスするには行セキュリティポリシーによって許可される必要がある。

行セキュリティポリシーは特定のコマンド、特定のロール、あるいはその両方に対して定義できる。基本的にはコマンドは ALL , ロールは会社単位になりそう。



```
CREATE POLICY user_policy ON users
    USING (user_name = current_user);
```

上のような書き方をすると、一つのポリシーで全てのユーザーに適用させることができる。

しかし、マルチテナント管理において、PK ID が数値やランダム文字列の場合 `tenant_id = current_user` とする必要があるため、dbユーザー名もそれに合わせる必要あり。（dbユーザー名が1とかちょっと嫌よねって話。）

とりあえずトライ。

テーブル作成してRLSを適用

```
CREATE TABLE accounts (manager text, company text, contact_email text);

ALTER TABLE accounts ENABLE ROW LEVEL SECURITY;
```

```
xdb=# \d accounts
                Table "public.accounts"
    Column     | Type | Collation | Nullable | Default
---------------+------+-----------+----------+---------
 manager       | text |           |          |
 company       | text |           |          |
 contact_email | text |           |          |
Policies (row security enabled): (none)

```

create a role

```
CREATE ROLE manager
```

create policy

```
create policy account_managers on accounts to manager using (manager = current_user);
```

```
xdb=# \d accounts
                Table "public.accounts"
    Column     | Type | Collation | Nullable | Default
---------------+------+-----------+----------+---------
 manager       | text |           |          |
 company       | text |           |          |
 contact_email | text |           |          |
Policies:
    POLICY "account_managers"
      TO manager
      USING ((manager = (CURRENT_USER)::text))
```

ポリシーがついた。

このポリシーが適用されるユーザーしか見れないことを確認する。ユーザーがいないのでOK, NG用に二人作る。

ユーザー作るのはREPLではできないので一旦出る。

```
\q
```

create user

```
# createuser --interactive shacho
Shall the new role be a superuser? (y/n) n
Shall the new role be allowed to create databases? (y/n) n
Shall the new role be allowed to create more new roles? (y/n) n
createuser: could not connect to database postgres: FATAL:  role "root" does not exist

```

これやってて気づいたけど、postgresにおいては *ユーザー＝ロール* のよう。 `\du` - こんなコマンドがあったが、確かに、表示にはロールが示されている

```
xdb=# \du
                                   List of roles
 Role name |                         Attributes                         | Member of
-----------+------------------------------------------------------------+-----------
 manager   | Cannot login                                               | {}
 xuser     | Superuser, Create role, Create DB, Replication, Bypass RLS | {}

```

なんてややこしさ。

だから `CREATE ROLE manager` でもうすでにユーザー作ってたことになるんですねf**k。

とはいえ作成した `manager` には何も権限がないよう。試しにユーザー切り替えようとすると

```
xdb=# \connect - manager
FATAL:  role "manager" is not permitted to log in
Previous connection kept
```

なのでログイン権限与える。

```
xdb=# create role manager with login;
ERROR:  role "manager" already exists
```

もう存在するので作ろうとすると期待通りエラーになる。権限だけ与える。 `ALTER ROLE` をつかいます

```
alter role manager with login
```

okそうなので次。`manager` にユーザー切り替える。

あ、 `accounts` テーブルにレコードがないので適当に入れる。

テーブル構造（再掲）

```
xdb=# \d accounts
                Table "public.accounts"
    Column     | Type | Collation | Nullable | Default
---------------+------+-----------+----------+---------
 manager       | text |           |          |
 company       | text |           |          |
 contact_email | text |           |          |
Policies (row security enabled): (none)
```

insert

```
insert into accounts values ('hogeman', 'hogeinc', 'hoge@hoge.com');
insert into accounts values ('fugaman', 'fugainc', 'fuga@fuga');
insert into accounts values ('manager', '真面目な会社', 'manager@majime.com');
```

```
xdb=# select * from accounts;
 manager |   company    |   contact_email
---------+--------------+--------------------
 hogeman | hogeinc      | hoge@hoge.com
 fugaman | fugainc      | fuga@fuga
 manager | 真面目な会社 | manager@majime.com
(3 rows)
```

ok. `manager` にユーザースイッチして同様に取得してみる

```
xdb=# \connect - manager;
You are now connected to database "xdb" as user "manager".
xdb=> select * from accounts;
ERROR:  permission denied for table accounts
```

RLSのおかげで期待通りできません。しかし、自分がmanagerである行については行セキュリティポリシーによって許可されるはず。なので、どうやってとるの？

もう一度テーブルを確認する。

```
xdb=# \d accounts
                Table "public.accounts"
    Column     | Type | Collation | Nullable | Default
---------------+------+-----------+----------+---------
 manager       | text |           |          |
 company       | text |           |          |
 contact_email | text |           |          |
Policies:
    POLICY "account_managers"
      TO manager
      USING ((manager = (CURRENT_USER)::text))
```

問題なさそう。current_userを確認する。

```
xdb=> select current_user;
 current_user
--------------
 manager
(1 row)
```

良さそう。

てか、このユーザーの権限?だとselectもinsertも何もできないのか。

accountsテーブルにおける利用可能な全ての権限を、managerユーザに与えます。

```
xdb=> GRANT ALL PRIVILEGES ON accounts TO manager;
ERROR:  permission denied for table accounts

```

あー,このユーザーには権限付与権限がないのでスーパーユーザーにスイッチしてやります。成功。managerにreスイッチ。リトライ。

```
xdb=> select * from accounts;
 manager |   company    |   contact_email
---------+--------------+--------------------
 manager | 真面目な会社 | manager@majime.com
(1 row)
```

おお成功！自分がmanagerである列の情報しか取得できていません。

where句も指定してみる。

```
xdb=> select * from accounts where manager = 'manager';
 manager |   company    |   contact_email
---------+--------------+--------------------
 manager | 真面目な会社 | manager@majime.com
(1 row)

```

OK、いいですね〜。where区を指定しようがしまいがどちらでも構わないから、アプリケーションコードでメソッド単位でいちいち、 `where tenant_id = ?` する必要はない。

では、他のmanagerの列をselectしてみる。

```
xdb=> select * from accounts where manager = 'hogeman';
 manager | company | contact_email
---------+---------+---------------
(0 rows)
```

`hogeman` は確かに存在するけども、RLSによって切り捨てられていますね。エラーにはならずに返らないだけ。

一応、更新系も見ておく。

success patterns

```
xdb=> insert into accounts values ('manager', '真面目な会社2', 'manager@majime2.com');
INSERT 0 1

xdb=> select  * from accounts;
 manager |    company    |    contact_email
---------+---------------+---------------------
 manager | 真面目な会社  | manager@majime.com
 manager | 真面目な会社2 | manager@majime2.com
(2 rows)

xdb=> update accounts set company = 'upd majime 2' where contact_email = 'manager@majime2.com';;
UPDATE 1

xdb=> delete from accounts where company = 'upd majime 2';
DELETE 1

```

failure patterns

```
xdb=> insert into accounts values ('hogeman', '真面目な会社2', 'manager@majime2.com');
ERROR:  new row violates row-level security policy for table "accounts"

```

OK。insert処理の時はエラーになり、エラーメッセージにRLSが原因ということが書いています。

```
xdb=> update accounts set company = 'upd majime 2' where contact_email = 'hoge@hoge.com';
UPDATE 0

xdb=> delete from accounts where company = 'hogeinc';
DELETE 0

```

OK. update,deleteの時はNO ERROR. セレクト時と同じ挙動。



### RLSの設定まとめ
簡潔に手順。

```console
// 1. create table
CREATE TABLE accounts (manager text, company text, contact_email text);

// 2. enable RLS
ALTER TABLE accounts ENABLE ROW LEVEL SECURITY;

// 3. create role(user)
CREATE ROLE manager WITH LOGIN;

// 4. create policy
create policy account_managers on accounts to manager using (manager = current_user);

// 5. grant privileges to role(user)
GRANT ALL PRIVILEGES ON accounts TO manager;
``` 

いくつかのコマンドまとめられそうだけど、一旦こんな感じ。

## Extras

### Max Connections
`CONNECTION LIMIT connlimit`

ロール作成時に最大接続数を設定できる。


### Role Name
ロール名には制約がある。

https://www.postgresql.org/docs/current/sql-syntax-lexical.html

ちなみにこのsyntaxルールはロール名に限らず。

>SQL identifiers and key words must begin with a letter (a-z, but also letters with diacritical marks and non-Latin letters) or an underscore (_). Subsequent characters in an identifier or key word can be letters, underscores, digits (0-9), or dollar signs ($).

注意

>Note that dollar signs are not allowed in identifiers according to the letter of the SQL standard, so their use might render applications less portable. The SQL standard will not define a key word that contains digits or starts or ends with an underscore, so identifiers of this form are safe against possible conflict with future extensions of the standard.

今回マルチテナントでのRLS実装においては、ポリシーを

```
CREATE POLICY {policy} ON tenants USING(id = current_user)
```

な感じにしたら、テナント単位でポリシー作らなくて良くなるので、
`hoge_1` , `hoge$1` みたいにして、最後のtenant_idを抽出、ポリシーのルールに使用というかんじにしたい。抽出ができるのかは未明。

できないようならテナントごとにポリシー作って当てる感じになると思う。まあ上できなそうだしこっちになりそう。いか、

```
CREATE POLICY {policy} ON tenants USING(id = 3) TO {role}
```