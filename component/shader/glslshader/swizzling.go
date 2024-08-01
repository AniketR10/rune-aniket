// nolint:unused
// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

// nolint: unused
package glslshader

// 2D Vectors -------------------------------------------------------------------------
// vec2D 2-component swizzling methods
func (v vec2D) xx() vec2D { return vec2D{v.x, v.x} }
func (v vec2D) yx() vec2D { return vec2D{v.y, v.x} }
func (v vec2D) yy() vec2D { return vec2D{v.y, v.y} }

// vec2D 3-component swizzling methods
func (v vec2D) xxx() vec3D { return vec3D{v.x, v.x, v.x} }
func (v vec2D) xxy() vec3D { return vec3D{v.x, v.x, v.y} }
func (v vec2D) xyx() vec3D { return vec3D{v.x, v.y, v.x} }
func (v vec2D) xyy() vec3D { return vec3D{v.x, v.y, v.y} }
func (v vec2D) yxx() vec3D { return vec3D{v.y, v.x, v.x} }
func (v vec2D) yxy() vec3D { return vec3D{v.y, v.x, v.y} }
func (v vec2D) yyx() vec3D { return vec3D{v.y, v.y, v.x} }
func (v vec2D) yyy() vec3D { return vec3D{v.y, v.y, v.y} }

// 3D Vectors -------------------------------------------------------------------------
// vec3D 2-component swizzling methods
func (v vec3D) xx() vec2D { return vec2D{v.x, v.x} }
func (v vec3D) xy() vec2D { return vec2D{v.x, v.y} }
func (v vec3D) xz() vec2D { return vec2D{v.x, v.z} }
func (v vec3D) yx() vec2D { return vec2D{v.y, v.x} }
func (v vec3D) yy() vec2D { return vec2D{v.y, v.y} }
func (v vec3D) yz() vec2D { return vec2D{v.y, v.z} }
func (v vec3D) zx() vec2D { return vec2D{v.z, v.x} }
func (v vec3D) zy() vec2D { return vec2D{v.z, v.y} }
func (v vec3D) zz() vec2D { return vec2D{v.z, v.z} }

// vec3D 3-component swizzling methods
func (v vec3D) xxx() vec3D { return vec3D{v.x, v.x, v.x} }
func (v vec3D) xxy() vec3D { return vec3D{v.x, v.x, v.y} }
func (v vec3D) xxz() vec3D { return vec3D{v.x, v.x, v.z} }
func (v vec3D) xyx() vec3D { return vec3D{v.x, v.y, v.x} }
func (v vec3D) xyy() vec3D { return vec3D{v.x, v.y, v.y} }
func (v vec3D) xyz() vec3D { return vec3D{v.x, v.y, v.z} }
func (v vec3D) xzx() vec3D { return vec3D{v.x, v.z, v.x} }
func (v vec3D) xzy() vec3D { return vec3D{v.x, v.z, v.y} }
func (v vec3D) xzz() vec3D { return vec3D{v.x, v.z, v.z} }
func (v vec3D) yxx() vec3D { return vec3D{v.y, v.x, v.x} }
func (v vec3D) yxy() vec3D { return vec3D{v.y, v.x, v.y} }
func (v vec3D) yxz() vec3D { return vec3D{v.y, v.x, v.z} }
func (v vec3D) yyx() vec3D { return vec3D{v.y, v.y, v.x} }
func (v vec3D) yyy() vec3D { return vec3D{v.y, v.y, v.y} }
func (v vec3D) yyz() vec3D { return vec3D{v.y, v.y, v.z} }
func (v vec3D) yzx() vec3D { return vec3D{v.y, v.z, v.x} }
func (v vec3D) yzy() vec3D { return vec3D{v.y, v.z, v.y} }
func (v vec3D) yzz() vec3D { return vec3D{v.y, v.z, v.z} }
func (v vec3D) zxx() vec3D { return vec3D{v.z, v.x, v.x} }
func (v vec3D) zxy() vec3D { return vec3D{v.z, v.x, v.y} }
func (v vec3D) zxz() vec3D { return vec3D{v.z, v.x, v.z} }
func (v vec3D) zyx() vec3D { return vec3D{v.z, v.y, v.x} }
func (v vec3D) zyy() vec3D { return vec3D{v.z, v.y, v.y} }
func (v vec3D) zyz() vec3D { return vec3D{v.z, v.y, v.z} }
func (v vec3D) zzx() vec3D { return vec3D{v.z, v.z, v.x} }
func (v vec3D) zzy() vec3D { return vec3D{v.z, v.z, v.y} }
func (v vec3D) zzz() vec3D { return vec3D{v.z, v.z, v.z} }

// vec3D 4-component swizzling methods
func (v vec3D) xxxx() vec4D { return vec4D{v.x, v.x, v.x, v.x} }
func (v vec3D) xxxy() vec4D { return vec4D{v.x, v.x, v.x, v.y} }
func (v vec3D) xxxz() vec4D { return vec4D{v.x, v.x, v.x, v.z} }
func (v vec3D) xxyx() vec4D { return vec4D{v.x, v.x, v.y, v.x} }
func (v vec3D) xxyy() vec4D { return vec4D{v.x, v.x, v.y, v.y} }
func (v vec3D) xxyz() vec4D { return vec4D{v.x, v.x, v.y, v.z} }
func (v vec3D) xxzx() vec4D { return vec4D{v.x, v.x, v.z, v.x} }
func (v vec3D) xxzy() vec4D { return vec4D{v.x, v.x, v.z, v.y} }
func (v vec3D) xxzz() vec4D { return vec4D{v.x, v.x, v.z, v.z} }
func (v vec3D) xyxx() vec4D { return vec4D{v.x, v.y, v.x, v.x} }
func (v vec3D) xyxy() vec4D { return vec4D{v.x, v.y, v.x, v.y} }
func (v vec3D) xyxz() vec4D { return vec4D{v.x, v.y, v.x, v.z} }
func (v vec3D) xyyx() vec4D { return vec4D{v.x, v.y, v.y, v.x} }
func (v vec3D) xyyy() vec4D { return vec4D{v.x, v.y, v.y, v.y} }
func (v vec3D) xyyz() vec4D { return vec4D{v.x, v.y, v.y, v.z} }
func (v vec3D) xyzx() vec4D { return vec4D{v.x, v.y, v.z, v.x} }
func (v vec3D) xyzy() vec4D { return vec4D{v.x, v.y, v.z, v.y} }
func (v vec3D) xyzz() vec4D { return vec4D{v.x, v.y, v.z, v.z} }
func (v vec3D) xzxx() vec4D { return vec4D{v.x, v.z, v.x, v.x} }
func (v vec3D) xzxy() vec4D { return vec4D{v.x, v.z, v.x, v.y} }
func (v vec3D) xzxz() vec4D { return vec4D{v.x, v.z, v.x, v.z} }
func (v vec3D) xzyx() vec4D { return vec4D{v.x, v.z, v.y, v.x} }
func (v vec3D) xzyy() vec4D { return vec4D{v.x, v.z, v.y, v.y} }
func (v vec3D) xzyz() vec4D { return vec4D{v.x, v.z, v.y, v.z} }
func (v vec3D) xzzx() vec4D { return vec4D{v.x, v.z, v.z, v.x} }
func (v vec3D) xzzy() vec4D { return vec4D{v.x, v.z, v.z, v.y} }
func (v vec3D) xzzz() vec4D { return vec4D{v.x, v.z, v.z, v.z} }
func (v vec3D) yxxx() vec4D { return vec4D{v.y, v.x, v.x, v.x} }
func (v vec3D) yxxy() vec4D { return vec4D{v.y, v.x, v.x, v.y} }
func (v vec3D) yxxz() vec4D { return vec4D{v.y, v.x, v.x, v.z} }
func (v vec3D) yxyx() vec4D { return vec4D{v.y, v.x, v.y, v.x} }
func (v vec3D) yxyy() vec4D { return vec4D{v.y, v.x, v.y, v.y} }
func (v vec3D) yxyz() vec4D { return vec4D{v.y, v.x, v.y, v.z} }
func (v vec3D) yxzx() vec4D { return vec4D{v.y, v.x, v.z, v.x} }
func (v vec3D) yxzy() vec4D { return vec4D{v.y, v.x, v.z, v.y} }
func (v vec3D) yxzz() vec4D { return vec4D{v.y, v.x, v.z, v.z} }
func (v vec3D) yyxx() vec4D { return vec4D{v.y, v.y, v.x, v.x} }
func (v vec3D) yyxy() vec4D { return vec4D{v.y, v.y, v.x, v.y} }
func (v vec3D) yyxz() vec4D { return vec4D{v.y, v.y, v.x, v.z} }
func (v vec3D) yyyx() vec4D { return vec4D{v.y, v.y, v.y, v.x} }
func (v vec3D) yyyy() vec4D { return vec4D{v.y, v.y, v.y, v.y} }
func (v vec3D) yyyz() vec4D { return vec4D{v.y, v.y, v.y, v.z} }
func (v vec3D) yyzx() vec4D { return vec4D{v.y, v.y, v.z, v.x} }
func (v vec3D) yyzy() vec4D { return vec4D{v.y, v.y, v.z, v.y} }
func (v vec3D) yyzz() vec4D { return vec4D{v.y, v.y, v.z, v.z} }
func (v vec3D) yzxx() vec4D { return vec4D{v.y, v.z, v.x, v.x} }
func (v vec3D) yzxy() vec4D { return vec4D{v.y, v.z, v.x, v.y} }
func (v vec3D) yzxz() vec4D { return vec4D{v.y, v.z, v.x, v.z} }
func (v vec3D) yzyx() vec4D { return vec4D{v.y, v.z, v.y, v.x} }
func (v vec3D) yzyy() vec4D { return vec4D{v.y, v.z, v.y, v.y} }
func (v vec3D) yzyz() vec4D { return vec4D{v.y, v.z, v.y, v.z} }
func (v vec3D) yzzx() vec4D { return vec4D{v.y, v.z, v.z, v.x} }
func (v vec3D) yzzy() vec4D { return vec4D{v.y, v.z, v.z, v.y} }
func (v vec3D) yzzz() vec4D { return vec4D{v.y, v.z, v.z, v.z} }
func (v vec3D) zxxx() vec4D { return vec4D{v.z, v.x, v.x, v.x} }
func (v vec3D) zxxy() vec4D { return vec4D{v.z, v.x, v.x, v.y} }
func (v vec3D) zxxz() vec4D { return vec4D{v.z, v.x, v.x, v.z} }
func (v vec3D) zxyx() vec4D { return vec4D{v.z, v.x, v.y, v.x} }
func (v vec3D) zxyy() vec4D { return vec4D{v.z, v.x, v.y, v.y} }
func (v vec3D) zxyz() vec4D { return vec4D{v.z, v.x, v.y, v.z} }
func (v vec3D) zxzx() vec4D { return vec4D{v.z, v.x, v.z, v.x} }
func (v vec3D) zxzy() vec4D { return vec4D{v.z, v.x, v.z, v.y} }
func (v vec3D) zxzz() vec4D { return vec4D{v.z, v.x, v.z, v.z} }
func (v vec3D) zyxx() vec4D { return vec4D{v.z, v.y, v.x, v.x} }
func (v vec3D) zyxy() vec4D { return vec4D{v.z, v.y, v.x, v.y} }
func (v vec3D) zyxz() vec4D { return vec4D{v.z, v.y, v.x, v.z} }
func (v vec3D) zyyx() vec4D { return vec4D{v.z, v.y, v.y, v.x} }
func (v vec3D) zyyy() vec4D { return vec4D{v.z, v.y, v.y, v.y} }
func (v vec3D) zyyz() vec4D { return vec4D{v.z, v.y, v.y, v.z} }
func (v vec3D) zyzx() vec4D { return vec4D{v.z, v.y, v.z, v.x} }
func (v vec3D) zyzy() vec4D { return vec4D{v.z, v.y, v.z, v.y} }
func (v vec3D) zyzz() vec4D { return vec4D{v.z, v.y, v.z, v.z} }
func (v vec3D) zzxx() vec4D { return vec4D{v.z, v.z, v.x, v.x} }
func (v vec3D) zzxy() vec4D { return vec4D{v.z, v.z, v.x, v.y} }
func (v vec3D) zzxz() vec4D { return vec4D{v.z, v.z, v.x, v.z} }
func (v vec3D) zzyx() vec4D { return vec4D{v.z, v.z, v.y, v.x} }
func (v vec3D) zzyy() vec4D { return vec4D{v.z, v.z, v.y, v.y} }
func (v vec3D) zzyz() vec4D { return vec4D{v.z, v.z, v.y, v.z} }
func (v vec3D) zzzx() vec4D { return vec4D{v.z, v.z, v.z, v.x} }
func (v vec3D) zzzy() vec4D { return vec4D{v.z, v.z, v.z, v.y} }
func (v vec3D) zzzz() vec4D { return vec4D{v.z, v.z, v.z, v.z} }

// 4D Vectors -------------------------------------------------------------------------
// vec4D 2-component swizzling methods
func (v vec4D) xx() vec2D { return vec2D{v.x, v.x} }
func (v vec4D) xy() vec2D { return vec2D{v.x, v.y} }
func (v vec4D) xz() vec2D { return vec2D{v.x, v.z} }
func (v vec4D) xw() vec2D { return vec2D{v.x, v.w} }
func (v vec4D) yx() vec2D { return vec2D{v.y, v.x} }
func (v vec4D) yy() vec2D { return vec2D{v.y, v.y} }
func (v vec4D) yz() vec2D { return vec2D{v.y, v.z} }
func (v vec4D) yw() vec2D { return vec2D{v.y, v.w} }
func (v vec4D) zx() vec2D { return vec2D{v.z, v.x} }
func (v vec4D) zy() vec2D { return vec2D{v.z, v.y} }
func (v vec4D) zz() vec2D { return vec2D{v.z, v.z} }
func (v vec4D) zw() vec2D { return vec2D{v.z, v.w} }
func (v vec4D) wx() vec2D { return vec2D{v.w, v.x} }
func (v vec4D) wy() vec2D { return vec2D{v.w, v.y} }
func (v vec4D) wz() vec2D { return vec2D{v.w, v.z} }
func (v vec4D) ww() vec2D { return vec2D{v.w, v.w} }

// vec4D 3-component swizzling methods
func (v vec4D) xxx() vec3D { return vec3D{v.x, v.x, v.x} }
func (v vec4D) xxy() vec3D { return vec3D{v.x, v.x, v.y} }
func (v vec4D) xxz() vec3D { return vec3D{v.x, v.x, v.z} }
func (v vec4D) xxw() vec3D { return vec3D{v.x, v.x, v.w} }
func (v vec4D) xyx() vec3D { return vec3D{v.x, v.y, v.x} }
func (v vec4D) xyy() vec3D { return vec3D{v.x, v.y, v.y} }
func (v vec4D) xyz() vec3D { return vec3D{v.x, v.y, v.z} }
func (v vec4D) xyw() vec3D { return vec3D{v.x, v.y, v.w} }
func (v vec4D) xzx() vec3D { return vec3D{v.x, v.z, v.x} }
func (v vec4D) xzy() vec3D { return vec3D{v.x, v.z, v.y} }
func (v vec4D) xzz() vec3D { return vec3D{v.x, v.z, v.z} }
func (v vec4D) xzw() vec3D { return vec3D{v.x, v.z, v.w} }
func (v vec4D) xwx() vec3D { return vec3D{v.x, v.w, v.x} }
func (v vec4D) xwy() vec3D { return vec3D{v.x, v.w, v.y} }
func (v vec4D) xwz() vec3D { return vec3D{v.x, v.w, v.z} }
func (v vec4D) xww() vec3D { return vec3D{v.x, v.w, v.w} }
func (v vec4D) yxx() vec3D { return vec3D{v.y, v.x, v.x} }
func (v vec4D) yxy() vec3D { return vec3D{v.y, v.x, v.y} }
func (v vec4D) yxz() vec3D { return vec3D{v.y, v.x, v.z} }
func (v vec4D) yxw() vec3D { return vec3D{v.y, v.x, v.w} }
func (v vec4D) yyx() vec3D { return vec3D{v.y, v.y, v.x} }
func (v vec4D) yyy() vec3D { return vec3D{v.y, v.y, v.y} }
func (v vec4D) yyz() vec3D { return vec3D{v.y, v.y, v.z} }
func (v vec4D) yyw() vec3D { return vec3D{v.y, v.y, v.w} }
func (v vec4D) yzx() vec3D { return vec3D{v.y, v.z, v.x} }
func (v vec4D) yzy() vec3D { return vec3D{v.y, v.z, v.y} }
func (v vec4D) yzz() vec3D { return vec3D{v.y, v.z, v.z} }
func (v vec4D) yzw() vec3D { return vec3D{v.y, v.z, v.w} }
func (v vec4D) ywx() vec3D { return vec3D{v.y, v.w, v.x} }
func (v vec4D) ywy() vec3D { return vec3D{v.y, v.w, v.y} }
func (v vec4D) ywz() vec3D { return vec3D{v.y, v.w, v.z} }
func (v vec4D) yww() vec3D { return vec3D{v.y, v.w, v.w} }
func (v vec4D) zxx() vec3D { return vec3D{v.z, v.x, v.x} }
func (v vec4D) zxy() vec3D { return vec3D{v.z, v.x, v.y} }
func (v vec4D) zxz() vec3D { return vec3D{v.z, v.x, v.z} }
func (v vec4D) zxw() vec3D { return vec3D{v.z, v.x, v.w} }
func (v vec4D) zyx() vec3D { return vec3D{v.z, v.y, v.x} }
func (v vec4D) zyy() vec3D { return vec3D{v.z, v.y, v.y} }
func (v vec4D) zyz() vec3D { return vec3D{v.z, v.y, v.z} }
func (v vec4D) zyw() vec3D { return vec3D{v.z, v.y, v.w} }
func (v vec4D) zzx() vec3D { return vec3D{v.z, v.z, v.x} }
func (v vec4D) zzy() vec3D { return vec3D{v.z, v.z, v.y} }
func (v vec4D) zzz() vec3D { return vec3D{v.z, v.z, v.z} }
func (v vec4D) zzw() vec3D { return vec3D{v.z, v.z, v.w} }
func (v vec4D) zwx() vec3D { return vec3D{v.z, v.w, v.x} }
func (v vec4D) zwy() vec3D { return vec3D{v.z, v.w, v.y} }
func (v vec4D) zwz() vec3D { return vec3D{v.z, v.w, v.z} }
func (v vec4D) zww() vec3D { return vec3D{v.z, v.w, v.w} }
func (v vec4D) wxx() vec3D { return vec3D{v.w, v.x, v.x} }
func (v vec4D) wxy() vec3D { return vec3D{v.w, v.x, v.y} }
func (v vec4D) wxz() vec3D { return vec3D{v.w, v.x, v.z} }
func (v vec4D) wxw() vec3D { return vec3D{v.w, v.x, v.w} }
func (v vec4D) wyx() vec3D { return vec3D{v.w, v.y, v.x} }
func (v vec4D) wyy() vec3D { return vec3D{v.w, v.y, v.y} }
func (v vec4D) wyz() vec3D { return vec3D{v.w, v.y, v.z} }
func (v vec4D) wyw() vec3D { return vec3D{v.w, v.y, v.w} }
func (v vec4D) wzx() vec3D { return vec3D{v.w, v.z, v.x} }
func (v vec4D) wzy() vec3D { return vec3D{v.w, v.z, v.y} }
func (v vec4D) wzz() vec3D { return vec3D{v.w, v.z, v.z} }
func (v vec4D) wzw() vec3D { return vec3D{v.w, v.z, v.w} }
func (v vec4D) wwx() vec3D { return vec3D{v.w, v.w, v.x} }
func (v vec4D) wwy() vec3D { return vec3D{v.w, v.w, v.y} }
func (v vec4D) wwz() vec3D { return vec3D{v.w, v.w, v.z} }
func (v vec4D) www() vec3D { return vec3D{v.w, v.w, v.w} }

// vec4D 4-component swizzling methods
func (v vec4D) xxxx() vec4D { return vec4D{v.x, v.x, v.x, v.x} }
func (v vec4D) xxxy() vec4D { return vec4D{v.x, v.x, v.x, v.y} }
func (v vec4D) xxxz() vec4D { return vec4D{v.x, v.x, v.x, v.z} }
func (v vec4D) xxxw() vec4D { return vec4D{v.x, v.x, v.x, v.w} }
func (v vec4D) xxyx() vec4D { return vec4D{v.x, v.x, v.y, v.x} }
func (v vec4D) xxyy() vec4D { return vec4D{v.x, v.x, v.y, v.y} }
func (v vec4D) xxyz() vec4D { return vec4D{v.x, v.x, v.y, v.z} }
func (v vec4D) xxyw() vec4D { return vec4D{v.x, v.x, v.y, v.w} }
func (v vec4D) xxzx() vec4D { return vec4D{v.x, v.x, v.z, v.x} }
func (v vec4D) xxzy() vec4D { return vec4D{v.x, v.x, v.z, v.y} }
func (v vec4D) xxzz() vec4D { return vec4D{v.x, v.x, v.z, v.z} }
func (v vec4D) xxzw() vec4D { return vec4D{v.x, v.x, v.z, v.w} }
func (v vec4D) xxwx() vec4D { return vec4D{v.x, v.x, v.w, v.x} }
func (v vec4D) xxwy() vec4D { return vec4D{v.x, v.x, v.w, v.y} }
func (v vec4D) xxwz() vec4D { return vec4D{v.x, v.x, v.w, v.z} }
func (v vec4D) xxww() vec4D { return vec4D{v.x, v.x, v.w, v.w} }
func (v vec4D) xyxx() vec4D { return vec4D{v.x, v.y, v.x, v.x} }
func (v vec4D) xyxy() vec4D { return vec4D{v.x, v.y, v.x, v.y} }
func (v vec4D) xyxz() vec4D { return vec4D{v.x, v.y, v.x, v.z} }
func (v vec4D) xyxw() vec4D { return vec4D{v.x, v.y, v.x, v.w} }
func (v vec4D) xyyx() vec4D { return vec4D{v.x, v.y, v.y, v.x} }
func (v vec4D) xyyy() vec4D { return vec4D{v.x, v.y, v.y, v.y} }
func (v vec4D) xyyz() vec4D { return vec4D{v.x, v.y, v.y, v.z} }
func (v vec4D) xyyw() vec4D { return vec4D{v.x, v.y, v.y, v.w} }
func (v vec4D) xyzx() vec4D { return vec4D{v.x, v.y, v.z, v.x} }
func (v vec4D) xyzy() vec4D { return vec4D{v.x, v.y, v.z, v.y} }
func (v vec4D) xyzz() vec4D { return vec4D{v.x, v.y, v.z, v.z} }
func (v vec4D) xyzw() vec4D { return vec4D{v.x, v.y, v.z, v.w} }
func (v vec4D) xywx() vec4D { return vec4D{v.x, v.y, v.w, v.x} }
func (v vec4D) xywy() vec4D { return vec4D{v.x, v.y, v.w, v.y} }
func (v vec4D) xywz() vec4D { return vec4D{v.x, v.y, v.w, v.z} }
func (v vec4D) xyww() vec4D { return vec4D{v.x, v.y, v.w, v.w} }
func (v vec4D) xzxx() vec4D { return vec4D{v.x, v.z, v.x, v.x} }
func (v vec4D) xzxy() vec4D { return vec4D{v.x, v.z, v.x, v.y} }
func (v vec4D) xzxz() vec4D { return vec4D{v.x, v.z, v.x, v.z} }
func (v vec4D) xzxw() vec4D { return vec4D{v.x, v.z, v.x, v.w} }
func (v vec4D) xzyx() vec4D { return vec4D{v.x, v.z, v.y, v.x} }
func (v vec4D) xzyy() vec4D { return vec4D{v.x, v.z, v.y, v.y} }
func (v vec4D) xzyz() vec4D { return vec4D{v.x, v.z, v.y, v.z} }
func (v vec4D) xzyw() vec4D { return vec4D{v.x, v.z, v.y, v.w} }
func (v vec4D) xzzx() vec4D { return vec4D{v.x, v.z, v.z, v.x} }
func (v vec4D) xzzy() vec4D { return vec4D{v.x, v.z, v.z, v.y} }
func (v vec4D) xzzz() vec4D { return vec4D{v.x, v.z, v.z, v.z} }
func (v vec4D) xzzw() vec4D { return vec4D{v.x, v.z, v.z, v.w} }
func (v vec4D) xzwx() vec4D { return vec4D{v.x, v.z, v.w, v.x} }
func (v vec4D) xzwy() vec4D { return vec4D{v.x, v.z, v.w, v.y} }
func (v vec4D) xzwz() vec4D { return vec4D{v.x, v.z, v.w, v.z} }
func (v vec4D) xzww() vec4D { return vec4D{v.x, v.z, v.w, v.w} }
func (v vec4D) xwxx() vec4D { return vec4D{v.x, v.w, v.x, v.x} }
func (v vec4D) xwxy() vec4D { return vec4D{v.x, v.w, v.x, v.y} }
func (v vec4D) xwxz() vec4D { return vec4D{v.x, v.w, v.x, v.z} }
func (v vec4D) xwxw() vec4D { return vec4D{v.x, v.w, v.x, v.w} }
func (v vec4D) xwyx() vec4D { return vec4D{v.x, v.w, v.y, v.x} }
func (v vec4D) xwyy() vec4D { return vec4D{v.x, v.w, v.y, v.y} }
func (v vec4D) xwyz() vec4D { return vec4D{v.x, v.w, v.y, v.z} }
func (v vec4D) xwyw() vec4D { return vec4D{v.x, v.w, v.y, v.w} }
func (v vec4D) xwzx() vec4D { return vec4D{v.x, v.w, v.z, v.x} }
func (v vec4D) xwzy() vec4D { return vec4D{v.x, v.w, v.z, v.y} }
func (v vec4D) xwzz() vec4D { return vec4D{v.x, v.w, v.z, v.z} }
func (v vec4D) xwzw() vec4D { return vec4D{v.x, v.w, v.z, v.w} }
func (v vec4D) xwwx() vec4D { return vec4D{v.x, v.w, v.w, v.x} }
func (v vec4D) xwwy() vec4D { return vec4D{v.x, v.w, v.w, v.y} }
func (v vec4D) xwwz() vec4D { return vec4D{v.x, v.w, v.w, v.z} }
func (v vec4D) xwww() vec4D { return vec4D{v.x, v.w, v.w, v.w} }
func (v vec4D) yxxx() vec4D { return vec4D{v.y, v.x, v.x, v.x} }
func (v vec4D) yxxy() vec4D { return vec4D{v.y, v.x, v.x, v.y} }
func (v vec4D) yxxz() vec4D { return vec4D{v.y, v.x, v.x, v.z} }
func (v vec4D) yxxw() vec4D { return vec4D{v.y, v.x, v.x, v.w} }
func (v vec4D) yxyx() vec4D { return vec4D{v.y, v.x, v.y, v.x} }
func (v vec4D) yxyy() vec4D { return vec4D{v.y, v.x, v.y, v.y} }
func (v vec4D) yxyz() vec4D { return vec4D{v.y, v.x, v.y, v.z} }
func (v vec4D) yxyw() vec4D { return vec4D{v.y, v.x, v.y, v.w} }
func (v vec4D) yxzx() vec4D { return vec4D{v.y, v.x, v.z, v.x} }
func (v vec4D) yxzy() vec4D { return vec4D{v.y, v.x, v.z, v.y} }
func (v vec4D) yxzz() vec4D { return vec4D{v.y, v.x, v.z, v.z} }
func (v vec4D) yxzw() vec4D { return vec4D{v.y, v.x, v.z, v.w} }
func (v vec4D) yxwx() vec4D { return vec4D{v.y, v.x, v.w, v.x} }
func (v vec4D) yxwy() vec4D { return vec4D{v.y, v.x, v.w, v.y} }
func (v vec4D) yxwz() vec4D { return vec4D{v.y, v.x, v.w, v.z} }
func (v vec4D) yxww() vec4D { return vec4D{v.y, v.x, v.w, v.w} }
func (v vec4D) yyxx() vec4D { return vec4D{v.y, v.y, v.x, v.x} }
func (v vec4D) yyxy() vec4D { return vec4D{v.y, v.y, v.x, v.y} }
func (v vec4D) yyxz() vec4D { return vec4D{v.y, v.y, v.x, v.z} }
func (v vec4D) yyxw() vec4D { return vec4D{v.y, v.y, v.x, v.w} }
func (v vec4D) yyyx() vec4D { return vec4D{v.y, v.y, v.y, v.x} }
func (v vec4D) yyyy() vec4D { return vec4D{v.y, v.y, v.y, v.y} }
func (v vec4D) yyyz() vec4D { return vec4D{v.y, v.y, v.y, v.z} }
func (v vec4D) yyyw() vec4D { return vec4D{v.y, v.y, v.y, v.w} }
func (v vec4D) yyzx() vec4D { return vec4D{v.y, v.y, v.z, v.x} }
func (v vec4D) yyzy() vec4D { return vec4D{v.y, v.y, v.z, v.y} }
func (v vec4D) yyzz() vec4D { return vec4D{v.y, v.y, v.z, v.z} }
func (v vec4D) yyzw() vec4D { return vec4D{v.y, v.y, v.z, v.w} }
func (v vec4D) yywx() vec4D { return vec4D{v.y, v.y, v.w, v.x} }
func (v vec4D) yywy() vec4D { return vec4D{v.y, v.y, v.w, v.y} }
func (v vec4D) yywz() vec4D { return vec4D{v.y, v.y, v.w, v.z} }
func (v vec4D) yyww() vec4D { return vec4D{v.y, v.y, v.w, v.w} }
func (v vec4D) yzxx() vec4D { return vec4D{v.y, v.z, v.x, v.x} }
func (v vec4D) yzxy() vec4D { return vec4D{v.y, v.z, v.x, v.y} }
func (v vec4D) yzxz() vec4D { return vec4D{v.y, v.z, v.x, v.z} }
func (v vec4D) yzxw() vec4D { return vec4D{v.y, v.z, v.x, v.w} }
func (v vec4D) yzyx() vec4D { return vec4D{v.y, v.z, v.y, v.x} }
func (v vec4D) yzyy() vec4D { return vec4D{v.y, v.z, v.y, v.y} }
func (v vec4D) yzyz() vec4D { return vec4D{v.y, v.z, v.y, v.z} }
func (v vec4D) yzyw() vec4D { return vec4D{v.y, v.z, v.y, v.w} }
func (v vec4D) yzzx() vec4D { return vec4D{v.y, v.z, v.z, v.x} }
func (v vec4D) yzzy() vec4D { return vec4D{v.y, v.z, v.z, v.y} }
func (v vec4D) yzzz() vec4D { return vec4D{v.y, v.z, v.z, v.z} }
func (v vec4D) yzzw() vec4D { return vec4D{v.y, v.z, v.z, v.w} }
func (v vec4D) yzwx() vec4D { return vec4D{v.y, v.z, v.w, v.x} }
func (v vec4D) yzwy() vec4D { return vec4D{v.y, v.z, v.w, v.y} }
func (v vec4D) yzwz() vec4D { return vec4D{v.y, v.z, v.w, v.z} }
func (v vec4D) yzww() vec4D { return vec4D{v.y, v.z, v.w, v.w} }
func (v vec4D) ywxx() vec4D { return vec4D{v.y, v.w, v.x, v.x} }
func (v vec4D) ywxy() vec4D { return vec4D{v.y, v.w, v.x, v.y} }
func (v vec4D) ywxz() vec4D { return vec4D{v.y, v.w, v.x, v.z} }
func (v vec4D) ywxw() vec4D { return vec4D{v.y, v.w, v.x, v.w} }
func (v vec4D) ywyx() vec4D { return vec4D{v.y, v.w, v.y, v.x} }
func (v vec4D) ywyy() vec4D { return vec4D{v.y, v.w, v.y, v.y} }
func (v vec4D) ywyz() vec4D { return vec4D{v.y, v.w, v.y, v.z} }
func (v vec4D) ywyw() vec4D { return vec4D{v.y, v.w, v.y, v.w} }
func (v vec4D) ywzx() vec4D { return vec4D{v.y, v.w, v.z, v.x} }
func (v vec4D) ywzy() vec4D { return vec4D{v.y, v.w, v.z, v.y} }
func (v vec4D) ywzz() vec4D { return vec4D{v.y, v.w, v.z, v.z} }
func (v vec4D) ywzw() vec4D { return vec4D{v.y, v.w, v.z, v.w} }
func (v vec4D) ywwx() vec4D { return vec4D{v.y, v.w, v.w, v.x} }
func (v vec4D) ywwy() vec4D { return vec4D{v.y, v.w, v.w, v.y} }
func (v vec4D) ywwz() vec4D { return vec4D{v.y, v.w, v.w, v.z} }
func (v vec4D) ywww() vec4D { return vec4D{v.y, v.w, v.w, v.w} }
func (v vec4D) zxxx() vec4D { return vec4D{v.z, v.x, v.x, v.x} }
func (v vec4D) zxxy() vec4D { return vec4D{v.z, v.x, v.x, v.y} }
func (v vec4D) zxxz() vec4D { return vec4D{v.z, v.x, v.x, v.z} }
func (v vec4D) zxxw() vec4D { return vec4D{v.z, v.x, v.x, v.w} }
func (v vec4D) zxyx() vec4D { return vec4D{v.z, v.x, v.y, v.x} }
func (v vec4D) zxyy() vec4D { return vec4D{v.z, v.x, v.y, v.y} }
func (v vec4D) zxyz() vec4D { return vec4D{v.z, v.x, v.y, v.z} }
func (v vec4D) zxyw() vec4D { return vec4D{v.z, v.x, v.y, v.w} }
func (v vec4D) zxzx() vec4D { return vec4D{v.z, v.x, v.z, v.x} }
func (v vec4D) zxzy() vec4D { return vec4D{v.z, v.x, v.z, v.y} }
func (v vec4D) zxzz() vec4D { return vec4D{v.z, v.x, v.z, v.z} }
func (v vec4D) zxzw() vec4D { return vec4D{v.z, v.x, v.z, v.w} }
func (v vec4D) zxwx() vec4D { return vec4D{v.z, v.x, v.w, v.x} }
func (v vec4D) zxwy() vec4D { return vec4D{v.z, v.x, v.w, v.y} }
func (v vec4D) zxwz() vec4D { return vec4D{v.z, v.x, v.w, v.z} }
func (v vec4D) zxww() vec4D { return vec4D{v.z, v.x, v.w, v.w} }
func (v vec4D) zyxx() vec4D { return vec4D{v.z, v.y, v.x, v.x} }
func (v vec4D) zyxy() vec4D { return vec4D{v.z, v.y, v.x, v.y} }
func (v vec4D) zyxz() vec4D { return vec4D{v.z, v.y, v.x, v.z} }
func (v vec4D) zyxw() vec4D { return vec4D{v.z, v.y, v.x, v.w} }
func (v vec4D) zyyx() vec4D { return vec4D{v.z, v.y, v.y, v.x} }
func (v vec4D) zyyy() vec4D { return vec4D{v.z, v.y, v.y, v.y} }
func (v vec4D) zyyz() vec4D { return vec4D{v.z, v.y, v.y, v.z} }
func (v vec4D) zyyw() vec4D { return vec4D{v.z, v.y, v.y, v.w} }
func (v vec4D) zyzx() vec4D { return vec4D{v.z, v.y, v.z, v.x} }
func (v vec4D) zyzy() vec4D { return vec4D{v.z, v.y, v.z, v.y} }
func (v vec4D) zyzz() vec4D { return vec4D{v.z, v.y, v.z, v.z} }
func (v vec4D) zyzw() vec4D { return vec4D{v.z, v.y, v.z, v.w} }
func (v vec4D) zywx() vec4D { return vec4D{v.z, v.y, v.w, v.x} }
func (v vec4D) zywy() vec4D { return vec4D{v.z, v.y, v.w, v.y} }
func (v vec4D) zywz() vec4D { return vec4D{v.z, v.y, v.w, v.z} }
func (v vec4D) zyww() vec4D { return vec4D{v.z, v.y, v.w, v.w} }
func (v vec4D) zzxx() vec4D { return vec4D{v.z, v.z, v.x, v.x} }
func (v vec4D) zzxy() vec4D { return vec4D{v.z, v.z, v.x, v.y} }
func (v vec4D) zzxz() vec4D { return vec4D{v.z, v.z, v.x, v.z} }
func (v vec4D) zzxw() vec4D { return vec4D{v.z, v.z, v.x, v.w} }
func (v vec4D) zzyx() vec4D { return vec4D{v.z, v.z, v.y, v.x} }
func (v vec4D) zzyy() vec4D { return vec4D{v.z, v.z, v.y, v.y} }
func (v vec4D) zzyz() vec4D { return vec4D{v.z, v.z, v.y, v.z} }
func (v vec4D) zzyw() vec4D { return vec4D{v.z, v.z, v.y, v.w} }
func (v vec4D) zzzx() vec4D { return vec4D{v.z, v.z, v.z, v.x} }
func (v vec4D) zzzy() vec4D { return vec4D{v.z, v.z, v.z, v.y} }
func (v vec4D) zzzz() vec4D { return vec4D{v.z, v.z, v.z, v.z} }
func (v vec4D) zzzw() vec4D { return vec4D{v.z, v.z, v.z, v.w} }
func (v vec4D) zzwx() vec4D { return vec4D{v.z, v.z, v.w, v.x} }
func (v vec4D) zzwy() vec4D { return vec4D{v.z, v.z, v.w, v.y} }
func (v vec4D) zzwz() vec4D { return vec4D{v.z, v.z, v.w, v.z} }
func (v vec4D) zzww() vec4D { return vec4D{v.z, v.z, v.w, v.w} }
func (v vec4D) zwxx() vec4D { return vec4D{v.z, v.w, v.x, v.x} }
func (v vec4D) zwxy() vec4D { return vec4D{v.z, v.w, v.x, v.y} }
func (v vec4D) zwxz() vec4D { return vec4D{v.z, v.w, v.x, v.z} }
func (v vec4D) zwxw() vec4D { return vec4D{v.z, v.w, v.x, v.w} }
func (v vec4D) zwyx() vec4D { return vec4D{v.z, v.w, v.y, v.x} }
func (v vec4D) zwyy() vec4D { return vec4D{v.z, v.w, v.y, v.y} }
func (v vec4D) zwyz() vec4D { return vec4D{v.z, v.w, v.y, v.z} }
func (v vec4D) zwyw() vec4D { return vec4D{v.z, v.w, v.y, v.w} }
func (v vec4D) zwzx() vec4D { return vec4D{v.z, v.w, v.z, v.x} }
func (v vec4D) zwzy() vec4D { return vec4D{v.z, v.w, v.z, v.y} }
func (v vec4D) zwzz() vec4D { return vec4D{v.z, v.w, v.z, v.z} }
func (v vec4D) zwzw() vec4D { return vec4D{v.z, v.w, v.z, v.w} }
func (v vec4D) zwwx() vec4D { return vec4D{v.z, v.w, v.w, v.x} }
func (v vec4D) zwwy() vec4D { return vec4D{v.z, v.w, v.w, v.y} }
func (v vec4D) zwwz() vec4D { return vec4D{v.z, v.w, v.w, v.z} }
func (v vec4D) zwww() vec4D { return vec4D{v.z, v.w, v.w, v.w} }
func (v vec4D) wxxx() vec4D { return vec4D{v.w, v.x, v.x, v.x} }
func (v vec4D) wxxy() vec4D { return vec4D{v.w, v.x, v.x, v.y} }
func (v vec4D) wxxz() vec4D { return vec4D{v.w, v.x, v.x, v.z} }
func (v vec4D) wxxw() vec4D { return vec4D{v.w, v.x, v.x, v.w} }
func (v vec4D) wxyx() vec4D { return vec4D{v.w, v.x, v.y, v.x} }
func (v vec4D) wxyy() vec4D { return vec4D{v.w, v.x, v.y, v.y} }
func (v vec4D) wxyz() vec4D { return vec4D{v.w, v.x, v.y, v.z} }
func (v vec4D) wxyw() vec4D { return vec4D{v.w, v.x, v.y, v.w} }
func (v vec4D) wxzx() vec4D { return vec4D{v.w, v.x, v.z, v.x} }
func (v vec4D) wxzy() vec4D { return vec4D{v.w, v.x, v.z, v.y} }
func (v vec4D) wxzz() vec4D { return vec4D{v.w, v.x, v.z, v.z} }
func (v vec4D) wxzw() vec4D { return vec4D{v.w, v.x, v.z, v.w} }
func (v vec4D) wxwx() vec4D { return vec4D{v.w, v.x, v.w, v.x} }
func (v vec4D) wxwy() vec4D { return vec4D{v.w, v.x, v.w, v.y} }
func (v vec4D) wxwz() vec4D { return vec4D{v.w, v.x, v.w, v.z} }
func (v vec4D) wxww() vec4D { return vec4D{v.w, v.x, v.w, v.w} }
func (v vec4D) wyxx() vec4D { return vec4D{v.w, v.y, v.x, v.x} }
func (v vec4D) wyxy() vec4D { return vec4D{v.w, v.y, v.x, v.y} }
func (v vec4D) wyxz() vec4D { return vec4D{v.w, v.y, v.x, v.z} }
func (v vec4D) wyxw() vec4D { return vec4D{v.w, v.y, v.x, v.w} }
func (v vec4D) wyyx() vec4D { return vec4D{v.w, v.y, v.y, v.x} }
func (v vec4D) wyyy() vec4D { return vec4D{v.w, v.y, v.y, v.y} }
func (v vec4D) wyyz() vec4D { return vec4D{v.w, v.y, v.y, v.z} }
func (v vec4D) wyyw() vec4D { return vec4D{v.w, v.y, v.y, v.w} }
func (v vec4D) wyzx() vec4D { return vec4D{v.w, v.y, v.z, v.x} }
func (v vec4D) wyzy() vec4D { return vec4D{v.w, v.y, v.z, v.y} }
func (v vec4D) wyzz() vec4D { return vec4D{v.w, v.y, v.z, v.z} }
func (v vec4D) wyzw() vec4D { return vec4D{v.w, v.y, v.z, v.w} }
func (v vec4D) wywx() vec4D { return vec4D{v.w, v.y, v.w, v.x} }
func (v vec4D) wywy() vec4D { return vec4D{v.w, v.y, v.w, v.y} }
func (v vec4D) wywz() vec4D { return vec4D{v.w, v.y, v.w, v.z} }
func (v vec4D) wyww() vec4D { return vec4D{v.w, v.y, v.w, v.w} }
func (v vec4D) wzxx() vec4D { return vec4D{v.w, v.z, v.x, v.x} }
func (v vec4D) wzxy() vec4D { return vec4D{v.w, v.z, v.x, v.y} }
func (v vec4D) wzxz() vec4D { return vec4D{v.w, v.z, v.x, v.z} }
func (v vec4D) wzxw() vec4D { return vec4D{v.w, v.z, v.x, v.w} }
func (v vec4D) wzyx() vec4D { return vec4D{v.w, v.z, v.y, v.x} }
func (v vec4D) wzyy() vec4D { return vec4D{v.w, v.z, v.y, v.y} }
func (v vec4D) wzyz() vec4D { return vec4D{v.w, v.z, v.y, v.z} }
func (v vec4D) wzyw() vec4D { return vec4D{v.w, v.z, v.y, v.w} }
func (v vec4D) wzzx() vec4D { return vec4D{v.w, v.z, v.z, v.x} }
func (v vec4D) wzzy() vec4D { return vec4D{v.w, v.z, v.z, v.y} }
func (v vec4D) wzzz() vec4D { return vec4D{v.w, v.z, v.z, v.z} }
func (v vec4D) wzzw() vec4D { return vec4D{v.w, v.z, v.z, v.w} }
func (v vec4D) wzwx() vec4D { return vec4D{v.w, v.z, v.w, v.x} }
func (v vec4D) wzwy() vec4D { return vec4D{v.w, v.z, v.w, v.y} }
func (v vec4D) wzwz() vec4D { return vec4D{v.w, v.z, v.w, v.z} }
func (v vec4D) wzww() vec4D { return vec4D{v.w, v.z, v.w, v.w} }
func (v vec4D) wwxx() vec4D { return vec4D{v.w, v.w, v.x, v.x} }
func (v vec4D) wwxy() vec4D { return vec4D{v.w, v.w, v.x, v.y} }
func (v vec4D) wwxz() vec4D { return vec4D{v.w, v.w, v.x, v.z} }
func (v vec4D) wwxw() vec4D { return vec4D{v.w, v.w, v.x, v.w} }
func (v vec4D) wwyx() vec4D { return vec4D{v.w, v.w, v.y, v.x} }
func (v vec4D) wwyy() vec4D { return vec4D{v.w, v.w, v.y, v.y} }
func (v vec4D) wwyz() vec4D { return vec4D{v.w, v.w, v.y, v.z} }
func (v vec4D) wwyw() vec4D { return vec4D{v.w, v.w, v.y, v.w} }
func (v vec4D) wwzx() vec4D { return vec4D{v.w, v.w, v.z, v.x} }
func (v vec4D) wwzy() vec4D { return vec4D{v.w, v.w, v.z, v.y} }
func (v vec4D) wwzz() vec4D { return vec4D{v.w, v.w, v.z, v.z} }
func (v vec4D) wwzw() vec4D { return vec4D{v.w, v.w, v.z, v.w} }
func (v vec4D) wwwx() vec4D { return vec4D{v.w, v.w, v.w, v.x} }
func (v vec4D) wwwy() vec4D { return vec4D{v.w, v.w, v.w, v.y} }
func (v vec4D) wwwz() vec4D { return vec4D{v.w, v.w, v.w, v.z} }
func (v vec4D) wwww() vec4D { return vec4D{v.w, v.w, v.w, v.w} }
