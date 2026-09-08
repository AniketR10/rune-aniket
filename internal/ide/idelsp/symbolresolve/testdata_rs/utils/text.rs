pub fn slugify(input: &str) -> String {
    private_helper(input)
}

fn private_helper(input: &str) -> String {
    input.to_lowercase()
}