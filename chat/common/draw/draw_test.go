package draw

import (
	"reflect"
	"testing"
)

func TestParsePrompt(t *testing.T) {
	type args struct {
		prompt string
	}
	// base test
	df := getDefaultDataTXT2IMGReq()
	df.Prompt += "This is a test prompt!"
	// civitai test
	df2 := getDefaultDataTXT2IMGReq()
	df2.Prompt = "shukezouma, octane render, hdr, (hyperdetailed:1.15), (soft light, sharp:1.2), 1girl, beautiful girl, ultra detailed eyes, mature, plump, thick, rainbow painting drops, paint teardrops, woman made up from paint, entirely paint, splat, splash, long colored hair, kimono made from paint, ultra detailed texture kimono, rainbow paint kimono, paint bulb, paint drops, (hair ornaments, earrings, flowers hair ornaments, butterflies hair ornaments), outdoors, sakura trees"
	df2.NegativePrompt = "3d, cartoon, anime, sketches, (worst quality:2), (low quality:2), (normal quality:2), lowres, normal quality, ((monochrome)), ((grayscale)), skin spots, acnes, skin blemishes, bad anatomy, girl, loli, young, large breasts, red eyes, muscular, over saturated, over saturated, over saturated"
	df2.Steps = 35
	df2.SamplerName = "DPM++ 2M Karras"
	df2.Seed = 4146499890
	df2.CfgScale = 7
	df2.Height = 768
	df2.DenoisingStrength = 0.52

	tests := []struct {
		name string
		args args
		want TXT2IMGReq
	}{
		{
			name: "test1",
			args: args{prompt: "This is a test prompt!"},
			want: df,
		},
		{
			name: "test2",
			args: args{prompt: "shukezouma, octane render, hdr, (hyperdetailed:1.15), (soft light, sharp:1.2), 1girl, beautiful girl, ultra detailed eyes, mature, plump, thick, rainbow painting drops, paint teardrops, woman made up from paint, entirely paint, splat, splash, long colored hair, kimono made from paint, ultra detailed texture kimono, rainbow paint kimono, paint bulb, paint drops, (hair ornaments, earrings, flowers hair ornaments, butterflies hair ornaments), outdoors, sakura trees\nNegative prompt: 3d, cartoon, anime, sketches, (worst quality:2), (low quality:2), (normal quality:2), lowres, normal quality, ((monochrome)), ((grayscale)), skin spots, acnes, skin blemishes, bad anatomy, girl, loli, young, large breasts, red eyes, muscular, over saturated, over saturated, over saturated\nSteps: 35, Sampler: DPM++ 2M Karras, CFG scale: 7, Seed: 4146499890, Size: 512x768, Model hash: ec6f68ea63, Model: lyriel_v16, Denoising strength: 0.52, ENSD: 31337, Hires resize: 832x1280, Hires steps: 40, Hires upscaler: Latent"},
			want: df2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParsePrompt(tt.args.prompt); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParsePrompt() = %v, want %v", got, tt.want)
			}
		})
	}
}
