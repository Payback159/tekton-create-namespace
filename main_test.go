package main

import "testing"

func Test_validateAndTransformToK8sName(t *testing.T) {
	type args struct {
		namespace string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{name: "withSpecialChars", args: args{namespace: "feat/SOME-1234"}, want: "feat-some-1234", wantErr: false},
		{name: "onlySpecialChars", args: args{namespace: "&!-"}, want: "", wantErr: true},
		{name: "beginSpecialChars", args: args{namespace: "-feat/SOME-1234"}, want: "feat-some-1234", wantErr: false},
		{name: "endSpecialChars", args: args{namespace: "feat/SOME-1234-"}, want: "feat-some-1234", wantErr: false},
		{name: "beginAndEndSpecialChars", args: args{namespace: "-feat/SOME-1234-"}, want: "feat-some-1234", wantErr: false},
		{name: "withEmpty", args: args{namespace: ""}, want: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateAndTransformToK8sName(tt.args.namespace, '-')
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAndTransformToK8sName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("validateAndTransformToK8sName() got = %v, want %v", got, tt.want)
			}
		})
	}
}
