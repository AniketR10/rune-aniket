pub struct Binding {
    key: String,
}

pub struct Hint {
    binding: Option<Binding>,
}

pub struct Hints {
    enabled: Vec<Hint>,
}

pub struct Bindings(Vec<String>);

pub struct UiConfig {
    key_bindings: Option<Bindings>,
    fallback: Bindings,
    hints: Hints,
}

impl UiConfig {
    pub fn generate_hint_bindings(&mut self) {
        let key_bindings = if let Some(key_bindings) = self.key_bindings.as_mut() {
            &mut key_bindings.0
        } else {
            &mut self.fallback.0
        };

        for hint in &self.hints.enabled {
            let binding = match &hint.binding {
                Some(binding) => binding,
                None => continue,
            };

            key_bindings.push(binding.key.clone());
        }
    }
}
