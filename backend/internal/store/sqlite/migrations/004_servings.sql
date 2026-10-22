-- How many people a recipe serves; NULL when not given.
ALTER TABLE recipes ADD COLUMN servings INTEGER CHECK (servings BETWEEN 1 AND 100);
