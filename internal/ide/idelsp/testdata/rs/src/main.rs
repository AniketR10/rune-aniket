// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

//! Fixture crate driving the rust-analyzer e2e suite.
//!
//! The layout is kept deliberately small and stable so the e2e tests can
//! assert against fixed 0-based LSP positions.

/// Greeter holds a name and renders a greeting.
pub struct Greeter {
    pub name: String,
}

impl Greeter {
    /// new builds a Greeter from a name.
    pub fn new(name: &str) -> Greeter {
        Greeter {
            name: name.to_string(),
        }
    }

    /// greet renders the greeting string.
    pub fn greet(&self) -> String {
        format!("Hello, {}!", self.name)
    }
}

/// add adds two integers.
pub fn add(a: i32, b: i32) -> i32 {
    a + b
}

fn main() {
    let g = Greeter::new("World");
    println!("{}", g.greet());
    let result = add(1, 2);
    println!("{}", result);
}
