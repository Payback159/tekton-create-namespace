package main

import "testing"

func Test_validateAndNormalizeBranch(t *testing.T) {
	type args struct {
		branch string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{name: "withSpecialChars", args: args{branch: "feat/SOME-1234"}, want: "feat-some-1234", wantErr: false},
		{name: "onlySpecialChars", args: args{branch: "&!-"}, want: "", wantErr: true},
		{name: "beginSpecialChars", args: args{branch: "-feat/SOME-1234"}, want: "feat-some-1234", wantErr: false},
		{name: "endSpecialChars", args: args{branch: "feat/SOME-1234-"}, want: "feat-some-1234", wantErr: false},
		{name: "beginAndEndSpecialChars", args: args{branch: "feat/SOME-1234-"}, want: "feat-some-1234", wantErr: false},
		{name: "withEmpty", args: args{branch: ""}, want: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateAndNormalizeBranch(tt.args.branch)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAndNormalizeBranch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("validateAndNormalizeBranch() got = %v, want %v", got, tt.want)
			}
		})
	}
}
