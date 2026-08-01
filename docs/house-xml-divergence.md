# `otservbr-house.xml` vs `world-house.xml`

Os dois arquivos vivem lado a lado em `data-otservbr-global/world/` e descrevem o
mesmo mundo. O servidor carrega **apenas um**: o que o OTBM referencia em
`HouseFile`, hoje `otservbr-house.xml`.

Este documento existe porque 102 casas desse arquivo têm `clientid="0"`, e o
`clientid` é a única coisa que o cliente usa para exibir uma casa — o servidor
nunca envia o nome de uma casa, em lugar nenhum de `protocolgame.cpp`. O cliente
resolve nome, cidade, tamanho, aluguel e dono a partir do `clientid` contra os
dados que ele mesmo traz. Com `clientid="0"` não há o que exibir.

Levantado em 2026-08-01. Nada aqui foi corrigido; é um registro do estado atual.

## Resumo

| | `otservbr-house.xml` | `world-house.xml` |
|---|---|---|
| casas | 1029 | 984 |
| `clientid="0"` | **102** | 0 |
| houseids exclusivos | 102 | 57 |
| houseids em comum | 927 | 927 |
| guildhalls | 65 | 65 |

Três fatos que se encaixam:

1. **Os 927 houseids em comum têm `clientid` idêntico — zero divergências.** A
   base dos dois arquivos é a mesma.
2. **As 102 casas com `clientid="0"` são exatamente as 102 que só existem no
   `otservbr-house.xml`.** A correlação é perfeita, não aproximada.
3. **Mas só 62 dessas são casas realmente novas.** As outras 40 são casas
   antigas remapeadas: mesmo nome e mesma posição de entrada, houseid novo, e o
   `clientid` perdido no caminho.

Exemplo do padrão de remapeamento:

```
otservbr:  "Harbour Place 1 (Shop)"  houseid=3718  size=29  rent=1000000  clientid=0
world:     "Harbour Place 1 (Shop)"  houseid=2941  size=21  rent=800000   clientid=11404

otservbr:  "The City Wall 5a"        houseid=3737  size=14  rent=50000    clientid=0
world:     "The City Wall 5a"        houseid=3001  size=17  rent=50000    clientid=11001
```

Para as 62 casas novas o `clientid="0"` está **correto** — não existe casa
oficial correspondente. Para as 40 remapeadas é uma perda de dado, e o valor
certo é recuperável casando nome + posição de entrada com `world-house.xml`.

## Por que o mundo em execução mostra 1086 casas

O boot carrega 1029 do XML e o resto vem da tabela `houses`, gravada por um boot
anterior que usou `world-house.xml`. Os 57 houseids exclusivos daquele arquivo
sobrevivem no banco e continuam sendo enviados ao cliente.

## Casas presentes só no `otservbr-house.xml` (todas com `clientid="0"`)

### Remapeadas — mesma casa, houseid novo, clientid perdido (40)

O `clientid` da coluna final é o que a mesma casa tem em `world-house.xml`, casada por nome + posição de entrada.

| nome | houseid (otservbr) | entrada | houseid (world) | clientid recuperável |
|---|---|---|---|---|
| Alai Flats, Flat 01 | 3687 | 32377,32256,7 | 2881 | 10301 |
| Alai Flats, Flat 02 | 3688 | 32382,32256,7 | 2882 | 10302 |
| Alai Flats, Flat 03 | 3689 | 32374,32264,7 | 2883 | 10303 |
| Alai Flats, Flat 06 | 3692 | 32386,32268,7 | 2886 | 10306 |
| Alai Flats, Flat 07 | 3693 | 32382,32268,7 | 2887 | 10307 |
| Alai Flats, Flat 08 | 3694 | 32377,32268,7 | 2888 | 10308 |
| Alai Flats, Flat 11 | 3695 | 32382,32256,6 | 2889 | 10311 |
| Alai Flats, Flat 12 | 3696 | 32377,32256,6 | 2890 | 10312 |
| Alai Flats, Flat 16 | 3699 | 32386,32268,6 | 2894 | 10316 |
| Alai Flats, Flat 17 | 3700 | 32382,32268,6 | 2895 | 10317 |
| Alai Flats, Flat 18 | 3701 | 32377,32268,6 | 2896 | 10318 |
| Alai Flats, Flat 21 | 3702 | 32382,32256,5 | 2897 | 10320 |
| Alai Flats, Flat 22 | 3703 | 32377,32256,5 | 2898 | 10319 |
| Alai Flats, Flat 26 | 3706 | 32386,32268,5 | 2902 | 10324 |
| Alai Flats, Flat 27 | 3707 | 32382,32268,5 | 2901 | 10325 |
| Alai Flats, Flat 28 | 3708 | 32377,32268,5 | 2900 | 10326 |
| Beach Home Apartments, Flat 01 | 3709 | 32314,32245,7 | 2905 | 10201 |
| Beach Home Apartments, Flat 02 | 3710 | 32314,32240,7 | 2906 | 10202 |
| Beach Home Apartments, Flat 03 | 3711 | 32317,32235,7 | 2907 | 10203 |
| Beach Home Apartments, Flat 04 | 3712 | 32313,32235,7 | 2908 | 10204 |
| Beach Home Apartments, Flat 05 | 3713 | 32309,32235,7 | 2909 | 10205 |
| Beach Home Apartments, Flat 06 | 3714 | 32309,32243,7 | 2910 | 10206 |
| Beach Home Apartments, Flat 11 | 3715 | 32314,32243,6 | 2911 | 10211 |
| Beach Home Apartments, Flat 12 | 3716 | 32314,32238,6 | 2912 | 10212 |
| Harbour Place 1 (Shop) | 3718 | 32336,32222,7 | 2941 | 11404 |
| Harbour Place 2 (Shop) | 3719 | 32336,32210,7 | 2942 | 11401 |
| Mill Avenue 3 | 3722 | 32410,32184,7 | 2952 | 10903 |
| Sorcerer's Avenue 1a | 3723 | 32300,32254,7 | 2957 | 10501 |
| Sorcerer's Avenue Labs 2a | 3724 | 32297,32273,8 | 2961 | 10505 |
| Sunset Homes, Flat 01 | 3730 | 32333,32232,7 | 2964 | 10101 |
| Sunset Homes, Flat 02 | 3731 | 32333,32237,7 | 2965 | 10102 |
| Sunset Homes, Flat 03 | 3732 | 32334,32244,7 | 2966 | 10103 |
| The City Wall 3a | 3734 | 32423,32208,7 | 2978 | 11016 |
| The City Wall 5a | 3737 | 32416,32220,7 | 3001 | 11001 |
| The City Wall 5c | 3738 | 32416,32223,7 | 3002 | 11003 |
| The City Wall 5e | 3739 | 32416,32226,7 | 3003 | 11005 |
| The City Wall 7a | 3740 | 32419,32237,7 | 2995 | 11007 |
| The City Wall 7c | 3741 | 32413,32237,7 | 2994 | 11009 |
| The City Wall 7e | 3742 | 32413,32238,7 | 2997 | 11011 |
| The City Wall 7g | 3743 | 32419,32238,7 | 2996 | 11013 |

### Casas novas — sem contrapartida oficial, `clientid="0"` está correto (62)

| nome | houseid | entrada |
|---|---|---|
| Alai Flats, Flat 04 | 3690 | 32386,32260,7 |
| Alai Flats, Flat 05 | 3691 | 32386,32267,7 |
| Alai Flats, Flat 13 | 3697 | 32378,32268,6 |
| Alai Flats, Flat 15 | 3698 | 32386,32267,6 |
| Alai Flats, Flat 23 | 3704 | 32378,32268,5 |
| Alai Flats, Flat 25 | 3705 | 32386,32267,5 |
| Central Plaza 1 | 3788 | 32347,31791,7 |
| Darashia, Hostel 1 | 3759 | 33243,32485,6 |
| Darashia, Hostel 2 | 3760 | 33239,32485,6 |
| Darashia, Hostel 3 | 3761 | 33237,32485,6 |
| Darashia, Hostel 4 | 3762 | 33241,32485,6 |
| Darashia, Hostel 6 | 3763 | 33248,32483,6 |
| Darashia, Hostel Coverage | 3764 | 33242,32484,5 |
| Darashia, Loft 1 | 3756 | 33204,32468,7 |
| Darashia, Loft 2 | 3757 | 33201,32468,6 |
| Darashia, Loft 3 | 3758 | 33202,32467,6 |
| Farm Lane, 1st Floor (Shop) | 3717 | 32382,32232,7 |
| Farmor Ville, 1 | 3765 | 32524,31785,7 |
| Farmor Ville, 10 | 3774 | 32554,31794,7 |
| Farmor Ville, 11 | 3775 | 32549,31794,7 |
| Farmor Ville, 12 | 3776 | 32562,31779,7 |
| Farmor Ville, 13 | 3777 | 32567,31779,7 |
| Farmor Ville, 14 | 3778 | 32572,31779,7 |
| Farmor Ville, 15 | 3779 | 32562,31780,7 |
| Farmor Ville, 16 | 3780 | 32570,31780,7 |
| Farmor Ville, 17 | 3781 | 32560,31846,6 |
| Farmor Ville, 18 | 3782 | 32553,31798,4 |
| Farmor Ville, 2 | 3766 | 32535,31765,7 |
| Farmor Ville, 3 | 3767 | 32551,31777,7 |
| Farmor Ville, 4 | 3768 | 32543,31795,7 |
| Farmor Ville, 5 | 3769 | 32560,31802,7 |
| Farmor Ville, 6 | 3770 | 32553,31802,7 |
| Farmor Ville, 7 | 3771 | 32546,31802,7 |
| Farmor Ville, 8 | 3772 | 32539,31802,7 |
| Farmor Ville, 9 | 3773 | 32559,31794,7 |
| Galuna's Rent House 01 | 3751 | 32350,32244,6 |
| Galuna's Rent House 02 | 3752 | 32345,32235,6 |
| Galuna's Rent House 03 | 3753 | 32343,32236,6 |
| Harbour Flats, Flat 12a | 3783 | 32380,31840,6 |
| Harbour Flats, Flat 13a | 3784 | 32384,31840,6 |
| Harbour Flats, Flat 19c | 3785 | 32388,31840,6 |
| Harbour Place 0 | 3755 | 32334,32210,8 |
| Harbour Place 4 | 3744 | 32330,32226,6 |
| Harbour Place 5 | 3745 | 32331,32208,5 |
| Harbour Place 6 | 3746 | 32337,32209,5 |
| Harbour Place 7 | 3747 | 32341,32212,6 |
| Harbour Street 4 | 3720 | 32332,32257,7 |
| Magician's Alley 8a | 3786 | 32322,31805,7 |
| Magician's Alley 8b | 3787 | 32322,31805,7 |
| Main Street 9, 1st Floor (Shop) | 3721 | 32377,32224,7 |
| Sorcerer's Avenue Labs 2b | 3725 | 32293,32273,8 |
| Sorcerer's Avenue Labs 2c | 3726 | 32305,32274,8 |
| Sorcerer's Avenue Labs 2d | 3727 | 32301,32274,8 |
| Sorcerer's Avenue Labs 2e | 3728 | 32297,32274,8 |
| Sorcerer's Avenue Labs 2f | 3729 | 32293,32274,8 |
| Thais Barn 01 | 3748 | 32401,32231,7 |
| Thais Barn 02 | 3749 | 32403,32231,7 |
| Thais Headlight Home | 3750 | 32344,32266,6 |
| The City Wall 1a | 3733 | 32422,32190,7 |
| The City Wall 3b | 3735 | 32423,32204,7 |
| The City Wall 3c | 3736 | 32423,32199,7 |
| The City Wall 8 | 3754 | 32413,32247,6 |

## Casas presentes só no `world-house.xml` (57)

Removidas pelo OTServBR. Ainda aparecem no mundo em execução porque ficaram gravadas na tabela `houses` de um boot anterior.

| nome | houseid | entrada | clientid |
|---|---|---|---|
| Alai Flats, Flat 01 | 2881 | 32377,32256,7 | 10301 |
| Alai Flats, Flat 02 | 2882 | 32382,32256,7 | 10302 |
| Alai Flats, Flat 03 | 2883 | 32374,32264,7 | 10303 |
| Alai Flats, Flat 04 | 2884 | 32386,32261,7 | 10304 |
| Alai Flats, Flat 05 | 2885 | 32386,32268,7 | 10305 |
| Alai Flats, Flat 06 | 2886 | 32386,32268,7 | 10306 |
| Alai Flats, Flat 07 | 2887 | 32382,32268,7 | 10307 |
| Alai Flats, Flat 08 | 2888 | 32377,32268,7 | 10308 |
| Alai Flats, Flat 11 | 2889 | 32382,32256,6 | 10311 |
| Alai Flats, Flat 12 | 2890 | 32377,32256,6 | 10312 |
| Alai Flats, Flat 13 | 2891 | 32377,32268,6 | 10313 |
| Alai Flats, Flat 15 | 2893 | 32386,32268,6 | 10315 |
| Alai Flats, Flat 16 | 2894 | 32386,32268,6 | 10316 |
| Alai Flats, Flat 17 | 2895 | 32382,32268,6 | 10317 |
| Alai Flats, Flat 18 | 2896 | 32377,32268,6 | 10318 |
| Alai Flats, Flat 21 | 2897 | 32382,32256,5 | 10320 |
| Alai Flats, Flat 22 | 2898 | 32377,32256,5 | 10319 |
| Alai Flats, Flat 23 | 2899 | 32377,32268,5 | 10321 |
| Alai Flats, Flat 25 | 2903 | 32386,32268,5 | 10323 |
| Alai Flats, Flat 26 | 2902 | 32386,32268,5 | 10324 |
| Alai Flats, Flat 27 | 2901 | 32382,32268,5 | 10325 |
| Alai Flats, Flat 28 | 2900 | 32377,32268,5 | 10326 |
| Beach Home Apartments, Flat 01 | 2905 | 32314,32245,7 | 10201 |
| Beach Home Apartments, Flat 02 | 2906 | 32314,32240,7 | 10202 |
| Beach Home Apartments, Flat 03 | 2907 | 32317,32235,7 | 10203 |
| Beach Home Apartments, Flat 04 | 2908 | 32313,32235,7 | 10204 |
| Beach Home Apartments, Flat 05 | 2909 | 32309,32235,7 | 10205 |
| Beach Home Apartments, Flat 06 | 2910 | 32309,32243,7 | 10206 |
| Beach Home Apartments, Flat 11 | 2911 | 32314,32243,6 | 10211 |
| Beach Home Apartments, Flat 12 | 2912 | 32314,32238,6 | 10212 |
| Farm Lane, 1st floor (Shop) | 2918 | 32382,32232,7 | 10702 |
| Harbour Lane 1 (Shop) | 2733 | 32366,31799,6 | 20701 |
| Harbour Place 1 (Shop) | 2941 | 32336,32222,7 | 11404 |
| Harbour Place 2 (Shop) | 2942 | 32336,32210,7 | 11401 |
| Harbour Place 4 | 2944 | 32332,32257,7 | 10602 |
| Main Street 9, 1st floor (Shop) | 2947 | 32377,32224,7 | 10802 |
| Mill Avenue 3 | 2952 | 32410,32184,7 | 10903 |
| Sorcerer's Avenue 1a | 2957 | 32300,32254,7 | 10501 |
| Sorcerer's Avenue Labs 2a | 2961 | 32297,32273,8 | 10505 |
| Sorcerer's Avenue Labs 2b | 2963 | 32301,32274,8 | 10508 |
| Sorcerer's Avenue Labs 2c | 2962 | 32297,32274,8 | 10510 |
| Sunset Homes, Flat 01 | 2964 | 32333,32232,7 | 10101 |
| Sunset Homes, Flat 02 | 2965 | 32333,32237,7 | 10102 |
| Sunset Homes, Flat 03 | 2966 | 32334,32244,7 | 10103 |
| The City Wall 1a | 2976 | 32422,32189,7 | 11022 |
| The City Wall 3a | 2978 | 32423,32208,7 | 11016 |
| The City Wall 3b | 2979 | 32423,32203,7 | 11017 |
| The City Wall 3c | 2980 | 32423,32198,7 | 11018 |
| The City Wall 5a | 3001 | 32416,32220,7 | 11001 |
| The City Wall 5c | 3002 | 32416,32223,7 | 11003 |
| The City Wall 5e | 3003 | 32416,32226,7 | 11005 |
| The City Wall 7a | 2995 | 32419,32237,7 | 11007 |
| The City Wall 7c | 2994 | 32413,32237,7 | 11009 |
| The City Wall 7e | 2997 | 32413,32238,7 | 11011 |
| The City Wall 7g | 2996 | 32419,32238,7 | 11013 |
| Theater Avenue 5 | 2747 | 32365,31782,6 | 20310 |
| Theater Avenue, Tower | 2703 | 32389,31787,7 | 20301 |

## Houseids comuns com campos divergentes (338 de 927)

Formato `campo: world -> otservbr`. Nenhum `clientid` diverge.

| houseid | nome | diferenças |
|---|---|---|
| 2904 | Alai Flats, Flat 24 | size:23->20 |
| 2913 | Beach Home Apartments, Flat 13 | size:19->17 |
| 2915 | Beach Home Apartments, Flat 15 | size:9->7 |
| 3054 | Blessed Shield Guildhall | size:250->171 |
| 2768 | Caretaker's Residence | size:298->317 |
| 2713 | Carlin Clanhall | size:287->296 |
| 3133 | Castle Shop 1 | size:38->65 |
| 3134 | Castle Shop 2 | size:38->66 |
| 3135 | Castle Shop 3 | size:38->72 |
| 3106 | Castle Street 1 | size:71->97 |
| 3102 | Castle Street 2 | size:35->54 |
| 3103 | Castle Street 3 | size:41->61 |
| 3104 | Castle Street 4 | size:40->64 |
| 3105 | Castle Street 5 | size:40->60 |
| 3092 | Castle, 3rd Floor, Flat 01 | size:15->28 |
| 3091 | Castle, 3rd Floor, Flat 02 | size:18->29 |
| 3088 | Castle, 3rd Floor, Flat 03 | size:13->30 |
| 3087 | Castle, 3rd Floor, Flat 04 | size:13->25 |
| 3090 | Castle, 3rd Floor, Flat 05 | size:18->25 |
| 3089 | Castle, 3rd Floor, Flat 06 | size:22->30 |
| 3086 | Castle, 3rd Floor, Flat 07 | size:17->29 |
| 3101 | Castle, 4th Floor, Flat 01 | size:14->20 |
| 3100 | Castle, 4th Floor, Flat 02 | size:18->25 |
| 3097 | Castle, 4th Floor, Flat 03 | size:14->29 |
| 3096 | Castle, 4th Floor, Flat 04 | size:14->24 |
| 3099 | Castle, 4th Floor, Flat 05 | size:18->25 |
| 3098 | Castle, 4th Floor, Flat 06 | size:21->29 |
| 3095 | Castle, 4th Floor, Flat 07 | size:17->29 |
| 3094 | Castle, 4th Floor, Flat 08 | size:22->41 |
| 3093 | Castle, 4th Floor, Flat 09 | size:17->24 |
| 3464 | Castle, Basement, Flat 01 | size:13->29 |
| 3465 | Castle, Basement, Flat 02 | size:13->20 |
| 3466 | Castle, Basement, Flat 03 | size:13->20 |
| 3468 | Castle, Basement, Flat 04 | size:13->20 |
| 3467 | Castle, Basement, Flat 05 | size:13->24 |
| 3469 | Castle, Basement, Flat 06 | size:13->24 |
| 3470 | Castle, Basement, Flat 07 | size:13->20 |
| 3472 | Castle, Basement, Flat 08 | size:13->20 |
| 3471 | Castle, Basement, Flat 09 | size:13->24 |
| 3085 | Castle, Residence | size:104->80 |
| 3136 | Central Circle 1 | size:76->111 |
| 3137 | Central Circle 2 | size:90->104 |
| 3138 | Central Circle 3 | size:99->108 |
| 3139 | Central Circle 4 | size:97->110 |
| 3140 | Central Circle 5 | size:99->115 |
| 3143 | Central Circle 6 (Shop) | size:101->127 |
| 3142 | Central Circle 7 (Shop) | size:101->120 |
| 3141 | Central Circle 8 (Shop) | size:101->124 |
| 3144 | Central Circle 9a | size:23->35 |
| 3145 | Central Circle 9b | size:23->24 |
| 2731 | Central Plaza 1 (Shop) | size:19->23 |
| 2730 | Central Plaza 2 (Shop) | size:15->23 |
| 2729 | Central Plaza 3 (Shop) | size:17->28 |
| 3018 | Dagger Alley 1 | size:103->68 |
| 3353 | Darashia 1, Flat 01 | size:29->48 |
| 3352 | Darashia 1, Flat 02 | size:26->41 |
| 3350 | Darashia 1, Flat 03 | size:65->95 |
| 3351 | Darashia 1, Flat 04 | size:28->42 |
| 3354 | Darashia 1, Flat 05 | size:29->47 |
| 3358 | Darashia 1, Flat 11 | size:27->28 |
| 3355 | Darashia 1, Flat 12 | size:46->51 |
| 3357 | Darashia 1, Flat 14 | size:69->68 |
| 3337 | Darashia 2, Flat 01 | size:29->48 |
| 3336 | Darashia 2, Flat 02 | size:26->42 |
| 3335 | Darashia 2, Flat 03 | size:31->41 |
| 3338 | Darashia 2, Flat 04 | size:14->24 |
| 3339 | Darashia 2, Flat 05 | size:31->48 |
| 3340 | Darashia 2, Flat 06 | size:14->24 |
| 3341 | Darashia 2, Flat 07 | size:29->47 |
| 3348 | Darashia 2, Flat 11 | size:27->28 |
| 3349 | Darashia 2, Flat 12 | size:13->17 |
| 3342 | Darashia 2, Flat 13 | size:31->32 |
| 3343 | Darashia 2, Flat 14 | size:14->17 |
| 3344 | Darashia 2, Flat 15 | size:30->34 |
| 3345 | Darashia 2, Flat 16 | size:18->19 |
| 3346 | Darashia 2, Flat 17 | size:27->26 |
| 3314 | Darashia 3, Flat 01 | size:27->39 |
| 3316 | Darashia 3, Flat 02 | size:41->66 |
| 3318 | Darashia 3, Flat 03 | size:28->42 |
| 3317 | Darashia 3, Flat 04 | size:39->66 |
| 3315 | Darashia 3, Flat 05 | size:26->39 |
| 3320 | Darashia 3, Flat 11 | size:27->33 |
| 3319 | Darashia 3, Flat 12 | size:56->64 |
| 3322 | Darashia 3, Flat 13 | size:27->28 |
| 3321 | Darashia 3, Flat 14 | size:59->69 |
| 3374 | Darashia 4, Flat 01 | size:31->47 |
| 3370 | Darashia 4, Flat 02 | size:44->65 |
| 3371 | Darashia 4, Flat 03 | size:27->42 |
| 3372 | Darashia 4, Flat 04 | size:45->72 |
| 3373 | Darashia 4, Flat 05 | size:30->46 |
| 3376 | Darashia 4, Flat 11 | size:26->27 |
| 3377 | Darashia 4, Flat 13 | size:44->43 |
| 3378 | Darashia 4, Flat 14 | size:46->44 |
| 3360 | Darashia 5, Flat 01 | size:29->46 |
| 3359 | Darashia 5, Flat 02 | size:41->59 |
| 3363 | Darashia 5, Flat 03 | size:27->42 |
| 3362 | Darashia 5, Flat 04 | size:42->66 |
| 3361 | Darashia 5, Flat 05 | size:29->46 |
| 3366 | Darashia 5, Flat 13 | size:42->43 |
| 3367 | Darashia 5, Flat 14 | size:38->40 |
| 3368 | Darashia 6a | size:67->97 |
| 3369 | Darashia 6b | size:80->61 |
| 3379 | Darashia 7, Flat 01 | size:26->38 |
| 3380 | Darashia 7, Flat 02 | size:27->42 |
| 3381 | Darashia 7, Flat 03 | size:65->108 |
| 3383 | Darashia 7, Flat 04 | size:27->36 |
| 3382 | Darashia 7, Flat 05 | size:27->46 |
| 3385 | Darashia 7, Flat 11 | size:26->27 |
| 3384 | Darashia 7, Flat 12 | size:60->65 |
| 3386 | Darashia 7, Flat 14 | size:60->61 |
| 3323 | Darashia 8, Flat 01 | size:55->77 |
| 3463 | Darashia 8, Flat 02 | size:76->114 |
| 3327 | Darashia 8, Flat 03 | size:105->162 |
| 3326 | Darashia 8, Flat 04 | size:63->90 |
| 3325 | Darashia 8, Flat 05 | size:58->85 |
| 3331 | Darashia 8, Flat 13 | size:46->43 |
| 3330 | Darashia 8, Flat 14 | size:42->40 |
| 3333 | Darashia, Eastern Guildhall | size:272->386 |
| 3332 | Darashia, Villa | size:120->178 |
| 3334 | Darashia, Western Guildhall | size:223->321 |
| 3008 | Dark Mansion | size:375->392 |
| 3017 | Dream Street 1 (Shop) | size:149->107 |
| 3019 | Dream Street 2 | size:113->80 |
| 3020 | Dream Street 3 | size:104->67 |
| 3027 | Dream Street 4 | size:128->89 |
| 2793 | East Lane 1a | size:62->80 |
| 2792 | East Lane 1b | size:43->40 |
| 2705 | East Lane 2 | size:93->102 |
| 3674 | Eastern House of Tranquility | size:313->332 |
| 3110 | Edron Flats, Flat 01 | size:11->24 |
| 3114 | Edron Flats, Flat 02 | size:20->36 |
| 3113 | Edron Flats, Flat 03 | size:11->19 |
| 3109 | Edron Flats, Flat 04 | size:10->25 |
| 3108 | Edron Flats, Flat 05 | size:10->20 |
| 3112 | Edron Flats, Flat 06 | size:11->19 |
| 3111 | Edron Flats, Flat 07 | size:11->20 |
| 3107 | Edron Flats, Flat 08 | size:10->19 |
| 3124 | Edron Flats, Flat 11 | size:32->38 |
| 3119 | Edron Flats, Flat 13 | size:22->25 |
| 3121 | Edron Flats, Flat 14 | size:31->36 |
| 3128 | Edron Flats, Flat 21 | size:20->22 |
| 3127 | Edron Flats, Flat 24 | size:22->24 |
| 3125 | Edron Flats, Flat 25 | size:31->37 |
| 3021 | Elm Street 1 | size:99->64 |
| 3023 | Elm Street 2 | size:98->65 |
| 3022 | Elm Street 3 | size:107->65 |
| 3024 | Elm Street 4 | size:108->63 |
| 2919 | Farm Lane, 2nd Floor (Shop) | size:17->24 |
| 2920 | Farm Lane, Basement (Shop) | size:21->24 |
| 3042 | Golden Axe Guildhall | size:344->231 |
| 2751 | Harbour Flats, Flat 11 | size:17->20 |
| 2755 | Harbour Flats, Flat 12 | size:33->12 entry:32378,31840,6->32376,31840,6 |
| 2752 | Harbour Flats, Flat 13 | size:17->20 |
| 2753 | Harbour Flats, Flat 15 | size:27->15 entry:32387,31836,6->32386,31836,6 |
| 2757 | Harbour Flats, Flat 16 | size:35->16 entry:32386,31840,6->32389,31836,6 |
| 2759 | Harbour Flats, Flat 21 | size:23->24 |
| 2760 | Harbour Flats, Flat 22 | size:30->28 |
| 2761 | Harbour Flats, Flat 23 | size:17->12 |
| 2706 | Harbour Lane 2a (Shop) | size:18->30 |
| 2707 | Harbour Lane 2b (Shop) | size:21->38 |
| 2708 | Harbour Lane 3 | size:92->99 |
| 2943 | Harbour Place 3 | size:88->100 |
| 2712 | House of Recreation | size:469->541 |
| 3039 | Iron Alley 1 | size:101->56 |
| 3040 | Iron Alley 2 | size:128->53 |
| 3053 | Iron Alley Watch, Lower | size:217->121 |
| 3052 | Iron Alley Watch, Upper | size:215->121 |
| 2710 | Lonely Sea Side Hostel | size:331->265 |
| 3034 | Loot Lane 1 (Shop) | size:159->109 |
| 2945 | Lower Swamp Lane 1 | size:80->87 |
| 2946 | Lower Swamp Lane 3 | size:80->87 |
| 3057 | Lucky Lane 1 (Shop) | size:220->163 |
| 3037 | Lucky Lane 2 (Tower) | size:216->121 |
| 3038 | Lucky Lane 3 (Tower) | size:216->121 |
| 3504 | Magic Academy, Flat 2 | size:26->27 |
| 3505 | Magic Academy, Flat 3 | size:26->30 |
| 3506 | Magic Academy, Flat 4 | size:26->28 |
| 3507 | Magic Academy, Flat 5 | size:26->28 |
| 3503 | Magic Academy, Guild | size:195->208 |
| 2717 | Magician's Alley 1 | size:23->28 |
| 2720 | Magician's Alley 1a | size:16->23 |
| 2719 | Magician's Alley 1b | size:16->15 |
| 2721 | Magician's Alley 1c | size:13->12 |
| 2722 | Magician's Alley 1d | size:16->14 |
| 2714 | Magician's Alley 4 | size:60->71 |
| 2727 | Magician's Alley 5a | size:30->20 entry:32326,31804,7->32325,31805,7 |
| 2725 | Magician's Alley 5b | entry:32325,31804,7->32325,31805,7 |
| 2723 | Magician's Alley 5c | size:25->24 entry:32322,31802,6->32320,31803,6 |
| 2724 | Magician's Alley 5f | size:28->30 entry:32322,31807,6->32320,31808,6 |
| 2709 | Magician's Alley 8 | size:31->41 |
| 2948 | Main Street 9a, 2nd floor (Shop) | size:15->18 |
| 2949 | Main Street 9b, 2nd floor (Shop) | size:27->32 |
| 3029 | Market Street 1 | size:220->158 |
| 3033 | Market Street 2 | size:173->117 |
| 3030 | Market Street 3 | size:127->91 |
| 3031 | Market Street 4 (Shop) | size:176->116 |
| 3032 | Market Street 5 (Shop) | size:230->142 |
| 3051 | Market Street 6 | size:186->100 |
| 3050 | Market Street 7 | size:90->62 |
| 2950 | Mill Avenue 1 (Shop) | size:28->35 |
| 2951 | Mill Avenue 2 (Shop) | size:47->54 |
| 2953 | Mill Avenue 4 | size:33->28 entry:32413,32189,6->32413,32187,6 |
| 2954 | Mill Avenue 5 | size:69->70 |
| 2769 | Moonkeep | size:298->450 |
| 3035 | Mystic Lane 1 | size:92->71 |
| 3036 | Mystic Lane 2 | size:119->71 |
| 3049 | Mystic Lane 3 (Tower) | size:214->121 |
| 2687 | Northern Street 1a | size:26->41 |
| 2739 | Northern Street 1b | size:25->22 |
| 2740 | Northern Street 1c | size:21->24 |
| 2738 | Northern Street 3b | size:22->23 |
| 2698 | Northern Street 5 | size:52->57 |
| 2699 | Northern Street 7 | size:44->49 |
| 3028 | Old Lighthouse | size:157->90 |
| 2689 | Park Lane 1a | size:36->50 |
| 2763 | Park Lane 1b | size:39->33 |
| 2691 | Park Lane 2 | size:28->42 |
| 2688 | Park Lane 3a | size:36->46 |
| 2809 | Park Lane 3b | size:29->28 |
| 2690 | Park Lane 4 | size:27->41 |
| 3084 | Paupers Palace, Flat 01 | size:15->11 |
| 3083 | Paupers Palace, Flat 02 | size:14->10 |
| 3082 | Paupers Palace, Flat 03 | size:11->9 |
| 3080 | Paupers Palace, Flat 04 | size:17->12 |
| 3079 | Paupers Palace, Flat 05 | size:9->8 |
| 3078 | Paupers Palace, Flat 06 | size:11->12 |
| 3081 | Paupers Palace, Flat 07 | size:14->13 |
| 3070 | Paupers Palace, Flat 11 | size:14->8 |
| 3075 | Paupers Palace, Flat 12 | size:25->13 |
| 3071 | Paupers Palace, Flat 13 | size:20->11 |
| 3076 | Paupers Palace, Flat 14 | size:25->13 |
| 3072 | Paupers Palace, Flat 15 | size:20->11 |
| 3077 | Paupers Palace, Flat 16 | size:30->13 |
| 3073 | Paupers Palace, Flat 17 | size:20->11 |
| 3074 | Paupers Palace, Flat 18 | size:20->8 |
| 3066 | Paupers Palace, Flat 21 | size:18->8 |
| 3065 | Paupers Palace, Flat 22 | size:19->11 |
| 3069 | Paupers Palace, Flat 23 | size:29->13 |
| 3064 | Paupers Palace, Flat 24 | size:19->11 |
| 3068 | Paupers Palace, Flat 25 | size:24->13 |
| 3063 | Paupers Palace, Flat 26 | size:19->11 |
| 3067 | Paupers Palace, Flat 27 | size:23->14 |
| 3062 | Paupers Palace, Flat 28 | size:13->9 |
| 3061 | Paupers Palace, Flat 31 | size:40->21 |
| 3060 | Paupers Palace, Flat 32 | size:50->23 |
| 3059 | Paupers Palace, Flat 33 | size:35->19 |
| 3058 | Paupers Palace, Flat 34 | size:59->39 |
| 3388 | Pirate Shipwreck 1 | size:187->183 |
| 3389 | Pirate Shipwreck 2 | size:276->230 |
| 3056 | Salvation Street 1 (Shop) | size:215->148 |
| 3045 | Salvation Street 2 | size:113->93 |
| 3046 | Salvation Street 3 | size:143->91 |
| 3025 | Seagull Walk 1 | size:169->129 |
| 3026 | Seagull Walk 2 | size:102->68 |
| 3043 | Silver Street 1 | size:108->63 |
| 3047 | Silver Street 2 | size:76->47 |
| 3048 | Silver Street 3 | size:82->50 |
| 3016 | Silver Street 4 | size:119->84 |
| 3146 | Sky Lane, Guild 1 | size:459->524 |
| 3149 | Sky Lane, Guild 2 | size:440->537 |
| 3148 | Sky Lane, Guild 3 | size:391->450 |
| 3147 | Sky Lane, Sea Tower | size:106->129 |
| 2959 | Sorcerer's Avenue 1b | size:19->23 |
| 2958 | Sorcerer's Avenue 5 (Shop) | size:54->28 |
| 3012 | Southern Thais Guildhall | size:374->527 |
| 3055 | Steel Home | size:388->305 |
| 3159 | Stronghold | size:194->184 |
| 2967 | Sunset Homes, Flat 11 | size:15->17 |
| 2968 | Sunset Homes, Flat 12 | size:15->16 |
| 2969 | Sunset Homes, Flat 13 | size:22->24 |
| 2970 | Sunset Homes, Flat 14 | size:17->16 |
| 2971 | Sunset Homes, Flat 21 | size:15->17 |
| 2972 | Sunset Homes, Flat 22 | size:15->16 |
| 2973 | Sunset Homes, Flat 23 | size:22->23 |
| 2974 | Sunset Homes, Flat 24 | size:17->16 |
| 2711 | Suntower | size:306->339 |
| 3041 | Swamp Watch | size:379->244 |
| 3014 | Thais Clanhall | size:206->213 |
| 2975 | Thais Hostel | rent:200000->5000000 size:129->250 |
| 2977 | The City Wall 1b | size:31->33 entry:32422,32189,6->32422,32186,6 |
| 2981 | The City Wall 3d | size:23->26 entry:32423,32208,6->32423,32206,6 |
| 2982 | The City Wall 3e | size:23->26 entry:32423,32203,6->32423,32201,6 |
| 2983 | The City Wall 3f | size:23->26 entry:32423,32198,6->32423,32196,6 |
| 2998 | The City Wall 5b | size:17->14 |
| 2999 | The City Wall 5d | size:15->16 |
| 2991 | The City Wall 7b | size:18->13 |
| 2992 | The City Wall 7d | size:22->18 |
| 2993 | The City Wall 7f | size:22->19 |
| 2990 | The City Wall 7h | size:18->14 |
| 2989 | The City Wall 9 | size:25->41 |
| 2718 | Theater Avenue 10 | size:29->45 |
| 2765 | Theater Avenue 11a | size:32->35 entry:32314,31781,5->32313,31781,5 |
| 2767 | Theater Avenue 11b | size:31->35 entry:32311,31781,5->32309,31781,5 |
| 2716 | Theater Avenue 12 | size:21->27 |
| 2715 | Theater Avenue 14 (Shop) | size:54->81 entry:32302,31801,7->32301,31791,7 |
| 2702 | Theater Avenue 6a | size:24->34 |
| 2736 | Theater Avenue 6b | size:25->20 |
| 2701 | Theater Avenue 6c | size:9->12 |
| 2735 | Theater Avenue 6d | size:7->9 |
| 2700 | Theater Avenue 6e | size:25->30 |
| 2734 | Theater Avenue 6f | size:24->17 |
| 2697 | Theater Avenue 7, Flat 01 | size:13->19 |
| 2696 | Theater Avenue 7, Flat 02 | size:15->20 |
| 2693 | Theater Avenue 7, Flat 03 | size:13->20 |
| 2692 | Theater Avenue 7, Flat 04 | size:15->19 |
| 2694 | Theater Avenue 7, Flat 05 | size:13->20 |
| 2695 | Theater Avenue 7, Flat 06 | size:13->19 |
| 2745 | Theater Avenue 7, Flat 11 | size:21->18 |
| 2744 | Theater Avenue 7, Flat 12 | size:14->15 |
| 2742 | Theater Avenue 7, Flat 13 | size:14->11 |
| 2741 | Theater Avenue 7, Flat 14 | size:13->12 |
| 2743 | Theater Avenue 7, Flat 15 | size:12->15 |
| 2746 | Theater Avenue 7, Flat 16 | size:16->12 |
| 2764 | Theater Avenue 8a | size:31->28 |
| 2985 | Upper Swamp Lane 10 | size:40->35 |
| 2984 | Upper Swamp Lane 12 | size:76->91 |
| 2988 | Upper Swamp Lane 2 | size:100->84 |
| 2987 | Upper Swamp Lane 4 | size:100->87 |
| 2986 | Upper Swamp Lane 8 | size:159->127 |
| 3044 | Valorous Venore | size:457->324 |
| 3004 | Warriors' Guildhall | size:334->347 |
| 3164 | Wood Avenue 1 | size:41->63 |
| 3153 | Wood Avenue 10a | size:35->55 |
| 3158 | Wood Avenue 10b | size:35->32 |
| 3150 | Wood Avenue 11 | size:165->207 |
| 3163 | Wood Avenue 2 | size:39->47 |
| 3161 | Wood Avenue 3 | size:39->43 |
| 3162 | Wood Avenue 4 | size:40->52 |
| 3166 | Wood Avenue 4a | size:33->40 |
| 3167 | Wood Avenue 4b | size:35->40 |
| 3165 | Wood Avenue 4c | size:41->45 |
| 3160 | Wood Avenue 5 | size:40->51 |
| 3155 | Wood Avenue 6a | size:34->51 |
| 3156 | Wood Avenue 6b | size:35->30 |
| 3152 | Wood Avenue 7 | size:145->154 |
| 3151 | Wood Avenue 8 | size:147->155 |
| 3154 | Wood Avenue 9a | size:33->52 |
| 3157 | Wood Avenue 9b | size:33->32 |
