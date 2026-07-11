use std::collections::HashMap;
use std::collections::BTreeMap;

fn main() {
    let mut m: HashMap<i32, i32> = HashMap::new();
    m.insert(1, 2);
    let mut b: BTreeMap<i32, i32> = BTreeMap::new();
    b.insert(3, 4);
    println!("{} {}", m.len(), b.len());
}
