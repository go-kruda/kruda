DROP TABLE IF EXISTS world;
CREATE TABLE world (
  id integer NOT NULL,
  randomnumber integer NOT NULL DEFAULT 0,
  PRIMARY KEY (id)
);

INSERT INTO world (id, randomnumber)
SELECT x.id, floor(random() * 10000 + 1)::integer
FROM generate_series(1, 10000) AS x(id);

DROP TABLE IF EXISTS fortune;
CREATE TABLE fortune (
  id integer NOT NULL,
  message varchar(2048) NOT NULL,
  PRIMARY KEY (id)
);

INSERT INTO fortune (id, message) VALUES
(1, 'fortune: No such file or directory'),
(2, 'A computer scientist is someone who fixes things that aren''t broken.'),
(3, 'After all is said and done, more is said than done.'),
(4, 'Any program that runs right is obsolete.'),
(5, 'A list is only as strong as its weakest link. — Donald Knuth'),
(6, 'Feature: A bug with seniority.'),
(7, 'Computers make very fast, very accurate mistakes.'),
(8, 'フレームワークのベンチマーク'),
(9, '<script>alert("This should not be displayed in a browser alert box.");</script>'),
(10, 'If Java had true garbage collection, most programs would delete themselves upon execution.'),
(11, 'http://www.techempower.com/blog/2013/03/28/framework-benchmarks/'),
(12, 'A bad random number generator: 1, 1, 1, 1, 1, 4.33e+67, 1, 1, 1');
