DROP TABLE IF EXISTS children;

CREATE TABLE `children` (
  `child_id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `child_name` varchar(50) NOT NULL,
  PRIMARY KEY (`child_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;