SELECT seqno, cid, name, "desc", coll, "key"
FROM pragma_index_xinfo(?, ?)
ORDER BY seqno;
