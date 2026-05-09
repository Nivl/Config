walk(
  if type == "array" and ([.[] | type] | all(. != "object" and . != "array")) then
    unique
  else
    .
  end
)
